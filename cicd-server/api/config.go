package api

import (
	"cicd-server/common"
	"cicd-server/config"
	"cicd-server/database"
	"cicd-server/models"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// UpdateProjectConfigRequest 更新项目配置请求（扁平结构，匹配CMDB格式）
type UpdateProjectConfigRequest struct {
	Project          string `json:"project"`           // 项目key（唯一标识）
	ProjectName      string `json:"project_name"`      // 项目中文名
	GitVue           string `json:"git_vue"`           // 前端git仓库
	GitBackend       string `json:"git_backend"`       // 后端git仓库
	UpdateFeishu     string `json:"update_feishu"`     // 发版通知地址
	NotifyFeishu     string `json:"notify_feishu"`     // 普通通知地址
	Description      string `json:"description"`       // 项目描述
	BackendTool      string `json:"backend_tool"`      // 后端工具（java17/java21等）
	FrontendTool     string `json:"frontend_tool"`     // 前端工具（node14/node16等）
	EnableSkyWalking bool   `json:"enable_skywalking"` // 是否启用skywalking
	CreatedBy        int64  `json:"created_by"`        // 创建人ID
	Timestamp        string `json:"timestamp"`         // 时间戳（用于防重放）
	Sign             string `json:"sign"`              // 签名（用于验证）
	UpdatedAt        string `json:"updated_at"`        // CMDB更新时间（仅接收，不使用）
	AgentURL         string `json:"agent_url"`         // Agent地址（仅接收，不使用）
	AlterFeishu      string `json:"alter_feishu"`      // 告警飞书（仅接收，不使用）
}

// BatchUpdateProjectConfigRequest 批量更新项目配置请求
type BatchUpdateProjectConfigRequest struct {
	Projects  map[string]models.ProjectConfig `json:"projects"`  // 项目配置列表
	Timestamp string                          `json:"timestamp"` // 时间戳（用于防重放）
	Sign      string                          `json:"sign"`      // 签名（用于验证）
}

// UpdateProjectConfig 更新单个项目配置（CMDB推送）
func UpdateProjectConfig(w http.ResponseWriter, r *http.Request) {
	// 只接受POST请求
	if r.Method != http.MethodPost {
		common.Logger.Warnf("配置更新请求方法错误: %s", r.Method)
		responseJSON(w, common.ErrorResponse(405, "方法不允许"), http.StatusMethodNotAllowed)
		return
	}

	common.Logger.Infof("收到项目配置更新请求 - RemoteAddr: %s", r.RemoteAddr)

	// 读取原始请求体（用于调试）
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		common.Logger.Errorf("读取请求体失败 - Error: %v", err)
		responseJSON(w, common.ErrorResponse(400, "读取请求体失败"), http.StatusBadRequest)
		return
	}

	// 打印原始加密请求体
	common.Logger.Infof("原始加密请求体 - Body: %s", string(bodyBytes))

	// 尝试解析外层JSON（判断是否加密）
	var encryptedWrapper struct {
		Data string `json:"data"`
	}

	var decryptedBytes []byte
	if err := json.Unmarshal(bodyBytes, &encryptedWrapper); err == nil && encryptedWrapper.Data != "" {
		// 有加密数据，需要解密
		common.Logger.Info("检测到加密数据，开始解密...")
		decryptedBytes, err = common.DecryptAndDecompress(encryptedWrapper.Data)
		if err != nil {
			common.Logger.Errorf("解密失败 - Error: %v", err)
			responseJSON(w, common.ErrorResponse(400, fmt.Sprintf("解密失败: %v", err)), http.StatusBadRequest)
			return
		}
		// 打印解密后的原始内容
		common.Logger.Infof("解密后的原始内容 - DecryptedBody: %s", string(decryptedBytes))
	} else {
		// 没有加密，直接使用原始数据
		common.Logger.Info("未检测到加密，使用原始数据")
		decryptedBytes = bodyBytes
	}

	// 解析请求体
	var req UpdateProjectConfigRequest
	if err := json.Unmarshal(decryptedBytes, &req); err != nil {
		common.Logger.Errorf("解析请求体失败 - Error: %v, DecryptedData: %s", err, string(decryptedBytes))
		responseJSON(w, common.ErrorResponse(400, fmt.Sprintf("无效的请求参数: %v", err)), http.StatusBadRequest)
		return
	}

	// 打印解析后的数据
	common.Logger.Infof("解析请求成功 - Project: %s, Timestamp: %s", req.Project, req.Timestamp)
	common.Logger.Infof("项目配置详情 - ProjectName: %s, GitBackend: %s, GitVue: %s, BackendTool: %s, FrontendTool: %s, UpdateFeishu: %s, NotifyFeishu: %s",
		req.ProjectName, req.GitBackend, req.GitVue, req.BackendTool, req.FrontendTool, req.UpdateFeishu, req.NotifyFeishu)

	// 验证参数
	if req.Project == "" {
		common.Logger.Error("项目key为空")
		responseJSON(w, common.ErrorResponse(400, "项目key不能为空"), http.StatusBadRequest)
		return
	}

	// 转换为ProjectConfig结构
	projectConfig := &models.ProjectConfig{
		ProjectName:      req.ProjectName,
		GitVue:           req.GitVue,
		GitBackend:       req.GitBackend,
		UpdateFeishu:     req.UpdateFeishu,
		NotifyFeishu:     req.NotifyFeishu,
		Description:      req.Description,
		BackendTool:      req.BackendTool,
		FrontendTool:     req.FrontendTool,
		EnableSkyWalking: req.EnableSkyWalking,
		CreatedBy:        req.CreatedBy,
	}

	// 验证签名（如果有提供的话）
	if req.Timestamp != "" || req.Sign != "" {
		if !verifySignature(req.Project, *projectConfig, req.Timestamp, req.Sign) {
			common.Logger.Errorf("签名验证失败 - Project: %s, Timestamp: %s, Sign: %s",
				req.Project, req.Timestamp, req.Sign)
			responseJSON(w, common.ErrorResponse(403, "签名验证失败"), http.StatusForbidden)
			return
		}
		common.Logger.Infof("签名验证成功 - Project: %s", req.Project)
	} else {
		common.Logger.Warnf("未提供签名，跳过验证 - Project: %s", req.Project)
	}

	// 更新到数据库（存在则更新，不存在则新增）
	if err := database.UpsertProject(req.Project, projectConfig); err != nil {
		common.Logger.Errorf("数据库更新失败 - Project: %s, Error: %v", req.Project, err)
		responseJSON(w, common.ErrorResponse(500, fmt.Sprintf("更新配置失败: %v", err)), http.StatusInternalServerError)
		return
	}

	common.Logger.Infof("数据库更新成功 - Project: %s", req.Project)

	// 刷新内存缓存
	if err := config.InitProjectsConfig(""); err != nil {
		common.Logger.Errorf("刷新配置缓存失败 - Error: %v", err)
		responseJSON(w, common.ErrorResponse(500, fmt.Sprintf("刷新缓存失败: %v", err)), http.StatusInternalServerError)
		return
	}

	common.Logger.Infof("项目配置更新完成 - Project: %s", req.Project)

	responseJSON(w, common.SuccessResponse(map[string]string{
		"message": fmt.Sprintf("项目 %s 配置更新成功", req.Project),
	}), http.StatusOK)
}

// BatchUpdateProjectConfig 批量更新项目配置（CMDB推送）
func BatchUpdateProjectConfig(w http.ResponseWriter, r *http.Request) {
	// 只接受POST请求
	if r.Method != http.MethodPost {
		common.Logger.Warnf("批量配置更新请求方法错误: %s", r.Method)
		responseJSON(w, common.ErrorResponse(405, "方法不允许"), http.StatusMethodNotAllowed)
		return
	}

	common.Logger.Infof("收到批量项目配置更新请求 - RemoteAddr: %s", r.RemoteAddr)

	// 读取原始请求体（用于调试）
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		common.Logger.Errorf("读取批量请求体失败 - Error: %v", err)
		responseJSON(w, common.ErrorResponse(400, "读取请求体失败"), http.StatusBadRequest)
		return
	}

	// 打印原始加密请求体
	common.Logger.Infof("批量原始加密请求体 - Body: %s", string(bodyBytes))

	// 尝试解析外层JSON（判断是否加密）
	var encryptedWrapper struct {
		Data string `json:"data"`
	}

	var decryptedBytes []byte
	if err := json.Unmarshal(bodyBytes, &encryptedWrapper); err == nil && encryptedWrapper.Data != "" {
		// 有加密数据，需要解密
		common.Logger.Info("检测到批量加密数据，开始解密...")
		decryptedBytes, err = common.DecryptAndDecompress(encryptedWrapper.Data)
		if err != nil {
			common.Logger.Errorf("批量解密失败 - Error: %v", err)
			responseJSON(w, common.ErrorResponse(400, fmt.Sprintf("解密失败: %v", err)), http.StatusBadRequest)
			return
		}
		// 打印解密后的原始内容
		common.Logger.Infof("批量解密后的原始内容 - DecryptedBody: %s", string(decryptedBytes))
	} else {
		// 没有加密，直接使用原始数据
		common.Logger.Info("未检测到加密，使用原始数据")
		decryptedBytes = bodyBytes
	}

	// 解析请求体
	var req BatchUpdateProjectConfigRequest
	if err := json.Unmarshal(decryptedBytes, &req); err != nil {
		common.Logger.Errorf("解析批量请求体失败 - Error: %v, DecryptedData: %s", err, string(decryptedBytes))
		responseJSON(w, common.ErrorResponse(400, fmt.Sprintf("无效的请求参数: %v", err)), http.StatusBadRequest)
		return
	}

	common.Logger.Infof("解析批量请求成功 - Count: %d, Timestamp: %s", len(req.Projects), req.Timestamp)

	// 验证参数
	if len(req.Projects) == 0 {
		common.Logger.Error("批量更新项目列表为空")
		responseJSON(w, common.ErrorResponse(400, "项目列表不能为空"), http.StatusBadRequest)
		return
	}

	// 验证签名
	if !verifyBatchSignature(req.Projects, req.Timestamp, req.Sign) {
		projectNames := make([]string, 0, len(req.Projects))
		for name := range req.Projects {
			projectNames = append(projectNames, name)
		}
		common.Logger.Errorf("批量签名验证失败 - Projects: %v, Timestamp: %s", projectNames, req.Timestamp)
		responseJSON(w, common.ErrorResponse(403, "签名验证失败"), http.StatusForbidden)
		return
	}

	common.Logger.Info("批量签名验证成功")

	// 批量更新到数据库
	if err := database.BatchUpsertProjects(req.Projects); err != nil {
		common.Logger.Errorf("批量数据库更新失败 - Error: %v", err)
		responseJSON(w, common.ErrorResponse(500, fmt.Sprintf("批量更新配置失败: %v", err)), http.StatusInternalServerError)
		return
	}

	common.Logger.Infof("批量数据库更新成功 - Count: %d", len(req.Projects))

	// 刷新内存缓存
	if err := config.InitProjectsConfig(""); err != nil {
		common.Logger.Errorf("刷新配置缓存失败 - Error: %v", err)
		responseJSON(w, common.ErrorResponse(500, fmt.Sprintf("刷新缓存失败: %v", err)), http.StatusInternalServerError)
		return
	}

	common.Logger.Infof("批量项目配置更新完成 - Count: %d", len(req.Projects))

	responseJSON(w, common.SuccessResponse(map[string]interface{}{
		"message": "批量更新项目配置成功",
		"count":   len(req.Projects),
	}), http.StatusOK)
}

// verifySignature 验证单个项目更新的签名
func verifySignature(name string, project models.ProjectConfig, timestamp, sign string) bool {
	// 获取加密盐
	salt := config.GetEncryptionSalt()
	if salt == "" {
		// 如果没有配置加密盐，跳过验证（不安全，建议配置）
		return true
	}

	// 构建签名字符串：name + git_backend + timestamp + salt
	signStr := fmt.Sprintf("%s|%s|%s|%s", name, project.GitBackend, timestamp, salt)

	// 计算MD5
	hash := md5.Sum([]byte(signStr))
	expectedSign := hex.EncodeToString(hash[:])

	return expectedSign == sign
}

// verifyBatchSignature 验证批量更新的签名
func verifyBatchSignature(projects map[string]models.ProjectConfig, timestamp, sign string) bool {
	// 获取加密盐
	salt := config.GetEncryptionSalt()
	if salt == "" {
		// 如果没有配置加密盐，跳过验证（不安全，建议配置）
		return true
	}

	// 将项目名称排序，确保签名一致性
	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sort.Strings(names)

	// 构建签名字符串：name1|name2|...|timestamp|salt
	signStr := strings.Join(names, "|") + "|" + timestamp + "|" + salt

	// 计算MD5
	hash := md5.Sum([]byte(signStr))
	expectedSign := hex.EncodeToString(hash[:])

	return expectedSign == sign
}
