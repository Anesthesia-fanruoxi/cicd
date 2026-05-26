package getProduct

import (
	"bytes"
	"cicd-server/config"
	"cicd-server/models"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// uploadResponse CMDB文件上传响应结构
type uploadResponse struct {
	Code int `json:"code"`
	Data struct {
		UUID        string `json:"uuid"`
		DownloadURL string `json:"download_url"`
		IsPrivate   bool   `json:"is_private"`
	} `json:"data"`
}

// ExecuteWebProduct 执行Web项目产物获取和上传
func ExecuteWebProduct(task *models.Task, projectName, gitCloneDir, productDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error) error {

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	addTaskLog(task, "开始获取Web项目产物")

	// 清理产品目录
	if err := os.RemoveAll(productDir); err != nil {
		addTaskLog(task, fmt.Sprintf("清理产品目录失败: %v", err))
	}
	if err := os.MkdirAll(productDir, 0755); err != nil {
		addTaskLog(task, fmt.Sprintf("创建产品目录失败: %v", err))
		return err
	}

	// 检查构建产物目录，根据category动态选择路径
	var basePath string
	if task.Category != "" {
		basePath = filepath.Join(gitCloneDir, task.Category)
		addTaskLog(task, fmt.Sprintf("使用category路径: %s", basePath))
	} else {
		basePath = gitCloneDir
	}

	distDir := filepath.Join(basePath, "dist")
	buildDir := filepath.Join(basePath, "build")

	var sourceDir string
	if _, err := os.Stat(distDir); err == nil {
		sourceDir = distDir
		addTaskLog(task, "发现dist目录，使用dist作为构建产物")
	} else if _, err := os.Stat(buildDir); err == nil {
		sourceDir = buildDir
		addTaskLog(task, "发现build目录，使用build作为构建产物")
	} else {
		return fmt.Errorf("未找到构建产物目录(dist或build)")
	}

	// 移动整个构建产物目录到product目录
	distInProduct := filepath.Join(productDir, "dist")
	moveCmd := fmt.Sprintf("mv %s %s", sourceDir, distInProduct)
	addTaskLog(task, fmt.Sprintf("移动构建产物: %s", moveCmd))

	if err := executeCommand(task, moveCmd); err != nil {
		addTaskLog(task, fmt.Sprintf("移动构建产物失败: %v", err))
		return err
	}

	// 使用任务的ImageTag作为版本标识（保持与回调一致）
	timestamp := task.ImageTag

	// 在product目录中创建zip压缩包，打包dist目录的内容
	var zipFileName string
	if task.Category != "" {
		zipFileName = fmt.Sprintf("%s-%s-%s.zip", projectName, task.Category, timestamp)
	} else {
		zipFileName = fmt.Sprintf("%s-%s.zip", projectName, timestamp)
	}
	zipFile := filepath.Join(productDir, zipFileName)
	zipCmd := fmt.Sprintf("cd %s/dist && zip -r ../%s .", productDir, zipFileName)
	addTaskLog(task, fmt.Sprintf("创建压缩包: %s", zipFile))

	if err := executeCommand(task, zipCmd); err != nil {
		addTaskLog(task, fmt.Sprintf("创建压缩包失败: %v", err))
		return err
	}

	// 通过HTTP上传到CMDB
	addTaskLog(task, "开始上传产物到CMDB...")
	downloadURL, err := uploadToCMDB(task, zipFile, addTaskLog)
	if err != nil {
		addTaskLog(task, fmt.Sprintf("上传到CMDB失败: %v", err))
		return err
	}

	// 设置任务产物的下载地址
	task.DownloadURL = downloadURL
	addTaskLog(task, fmt.Sprintf("Web项目产物上传完成，下载地址: %s", downloadURL))

	return nil
}

// uploadToCMDB 通过HTTP multipart将产物上传到CMDB
func uploadToCMDB(task *models.Task, zipFile string,
	addTaskLog func(*models.Task, string)) (string, error) {

	uploadURL := config.GetCMDBUploadURL()
	if uploadURL == "" {
		return "", fmt.Errorf("CMDB上传地址未配置")
	}

	addTaskLog(task, fmt.Sprintf("CMDB上传地址: %s", uploadURL))

	// 打开zip文件
	file, err := os.Open(zipFile)
	if err != nil {
		return "", fmt.Errorf("打开产物文件失败: %v", err)
	}
	defer file.Close()

	// 构建multipart表单
	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	// 添加文件字段
	fileWriter, err := multipartWriter.CreateFormFile("file", filepath.Base(zipFile))
	if err != nil {
		return "", fmt.Errorf("创建form文件字段失败: %v", err)
	}
	if _, err := io.Copy(fileWriter, file); err != nil {
		return "", fmt.Errorf("写入文件到form失败: %v", err)
	}

	// 添加目录字段
	if err := multipartWriter.WriteField("dir", "cicd-product"); err != nil {
		return "", fmt.Errorf("写入dir字段失败: %v", err)
	}

	// 添加公开文件标识（CICD产物使用公开下载）
	if err := multipartWriter.WriteField("is_private", "false"); err != nil {
		return "", fmt.Errorf("写入is_private字段失败: %v", err)
	}

	// 关闭multipart writer（写入结尾boundary）
	if err := multipartWriter.Close(); err != nil {
		return "", fmt.Errorf("关闭multipart writer失败: %v", err)
	}

	// 发送HTTP请求（30秒超时）
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	addTaskLog(task, fmt.Sprintf("正在上传文件: %s (大小: %d bytes)", filepath.Base(zipFile), requestBody.Len()))

	httpResp, err := client.Post(uploadURL, multipartWriter.FormDataContentType(), &requestBody)
	if err != nil {
		return "", fmt.Errorf("HTTP上传请求失败: %v", err)
	}
	defer httpResp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return "", fmt.Errorf("读取上传响应失败: %v", err)
	}

	addTaskLog(task, fmt.Sprintf("CMDB响应状态码: %d", httpResp.StatusCode))

	// 解析响应
	var resp uploadResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("解析CMDB响应失败: %v (原始响应: %s)", err, string(respBody))
	}

	if resp.Code != 200 {
		return "", fmt.Errorf("CMDB上传返回错误: code=%d", resp.Code)
	}

	if resp.Data.DownloadURL == "" {
		return "", fmt.Errorf("CMDB响应缺少download_url")
	}

	// CMDB返回的是相对路径，拼接host得到完整下载地址
	fullDownloadURL := config.GetCMDBConfig().BaseURL + resp.Data.DownloadURL

	addTaskLog(task, fmt.Sprintf("上传成功, UUID: %s", resp.Data.UUID))

	return fullDownloadURL, nil
}
