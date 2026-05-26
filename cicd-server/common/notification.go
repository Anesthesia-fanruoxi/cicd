package common

import (
	"bytes"
	"cicd-server/config"
	"cicd-server/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// UnifiedNotificationData 统一通知数据结构
type UnifiedNotificationData struct {
	// 通用字段
	IsStep    bool   `json:"isset"` // true=步骤通知, false=任务通知
	CreatedBy string `json:"created_by,omitempty"`

	// 任务通知字段
	ID           string `json:"id"`                      // 任务ID
	Name         string `json:"name,omitempty"`          // 项目简称
	ProjectName  string `json:"project_name,omitempty"`  // 项目名称
	Description  string `json:"description,omitempty"`   // 任务描述
	GitURL       string `json:"git_url,omitempty"`       // Git仓库地址
	UpdateFeishu string `json:"update_feishu,omitempty"` // 发版通知地址（替代ops飞书）
	NotifyFeishu string `json:"notify_feishu,omitempty"` // 普通通知地址（替代pro飞书）
	StartedAt    string `json:"started_at,omitempty"`    // 开始时间
	Type         string `json:"type,omitempty"`          // 任务类型
	TypeName     string `json:"type_name,omitempty"`     // 任务类型中文名称（前端/后端）
	FinishedAt   string `json:"finished_at"`             // 结束时间
	Status       string `json:"status,omitempty"`        // 状态 (running/complete/cancel)

	// 步骤通知字段
	Step           int     `json:"step,omitempty"`             // 步骤编号
	StepType       string  `json:"step_type,omitempty"`        // 步骤类型
	StepStartedAt  string  `json:"step_started_at,omitempty"`  // 步骤开始时间
	StepFinishedAt string  `json:"step_finished_at,omitempty"` // 步骤完成时间
	StepName       string  `json:"step_name,omitempty"`        // 步骤名称
	StepStatus     string  `json:"step_status,omitempty"`      // 步骤状态 (success/failed/cancel)
	Duration       float64 `json:"duration,omitempty"`         // 持续时间(秒)
	Remote         string  `json:"remote,omitempty"`           // 发起端标识（本服务固定为 server）
	LastDuration   float64 `json:"last_duration,omitempty"`    // 上次该步骤耗时(秒)
	EstimatedEnd   string  `json:"estimated_end,omitempty"`    // 预计结束时间
}

// EncryptedResponse 加密响应结构
type EncryptedResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data string `json:"data"` // 加密后的数据
}

// NotifyTaskCreate 通知任务创建
func NotifyTaskCreate(task *models.Task) error {
	// 检查必要字段
	if task == nil {
		return fmt.Errorf("任务对象为空")
	}
	if task.ProjectInfo == nil {
		return fmt.Errorf("项目信息为空")
	}

	// 处理开始时间
	var startedAtStr string
	if task.StartedAt != nil {
		startedAtStr = task.StartedAt.Format("2006-01-02 15:04:05")
	} else {
		// 任务创建时使用创建时间作为开始时间
		startedAtStr = task.CreatedAt.Format("2006-01-02 15:04:05")
	}

	// 根据任务类型选择Git仓库
	gitURL := task.ProjectInfo.GitBackend // 默认使用后端仓库
	if task.Type == "web" {
		gitURL = task.ProjectInfo.GitVue // Web项目使用前端仓库
	}

	// 获取任务类型中文名称
	typeName := getTypeName(task.Type)

	// 获取描述：优先使用请求传入的描述，否则使用项目配置中的描述
	description := task.Description
	if description == "" {
		description = task.ProjectInfo.Description
	}

	// 构建任务创建通知数据
	notificationData := UnifiedNotificationData{
		IsStep:       false, // 任务通知
		CreatedBy:    task.CreatedByName,
		ID:           task.ID,
		Name:         task.Name,
		ProjectName:  task.ProjectInfo.ProjectName + "-" + typeName,
		Description:  description,
		GitURL:       gitURL,
		UpdateFeishu: task.ProjectInfo.UpdateFeishu,
		NotifyFeishu: task.ProjectInfo.NotifyFeishu,
		StartedAt:    startedAtStr,
		Type:         task.Type, // 任务类型（web/double/single）
		TypeName:     typeName,  // 任务类型中文名称
		FinishedAt:   "",        // 创建时为空
		Status:       "running",
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(notificationData)
	if err != nil {
		Logger.Errorf("序列化任务创建数据失败: %v", err)
		return fmt.Errorf("序列化任务创建数据失败: %v", err)
	}

	// 压缩并加密数据
	encryptedData, err := CompressAndEncrypt(jsonData)
	if err != nil {
		Logger.Errorf("加密任务创建数据失败: %v", err)
		return fmt.Errorf("加密任务创建数据失败: %v", err)
	}

	return sendNotification(encryptedData, "任务创建")
}

// NotifyTaskComplete 通知任务完成
func NotifyTaskComplete(task *models.Task) error {
	// 检查必要字段
	if task == nil {
		return fmt.Errorf("任务对象为空")
	}
	if task.ProjectInfo == nil {
		return fmt.Errorf("项目信息为空")
	}
	if task.StartedAt == nil {
		return fmt.Errorf("任务开始时间为空")
	}
	if task.FinishedAt == nil {
		return fmt.Errorf("任务完成时间为空")
	}

	// 根据任务状态确定通知状态
	var status string
	switch task.Status {
	case models.TaskStatusSuccess:
		status = "complete"
	case models.TaskStatusFailed:
		status = "failed"
	case models.TaskStatusCanceled:
		status = "cancel"
	default:
		status = "complete"
	}

	// 根据任务类型选择Git仓库
	gitURL := task.ProjectInfo.GitBackend // 默认使用后端仓库
	if task.Type == "web" {
		gitURL = task.ProjectInfo.GitVue // Web项目使用前端仓库
	}

	// 获取任务类型中文名称
	typeName := getTypeName(task.Type)

	// 获取描述：优先使用请求传入的描述，否则使用项目配置中的描述
	description := task.Description
	if description == "" {
		description = task.ProjectInfo.Description
	}

	// 构建任务完成通知数据
	notificationData := UnifiedNotificationData{
		IsStep:       false, // 任务通知
		CreatedBy:    task.CreatedByName,
		ID:           task.ID,
		Name:         task.Name,
		ProjectName:  task.ProjectInfo.ProjectName + "-" + typeName,
		Description:  description,
		GitURL:       gitURL,
		UpdateFeishu: task.ProjectInfo.UpdateFeishu,
		NotifyFeishu: task.ProjectInfo.NotifyFeishu,
		StartedAt:    task.StartedAt.Format("2006-01-02 15:04:05"),
		Type:         task.Type, // 任务类型（web/double/single）
		TypeName:     typeName,  // 任务类型中文名称
		FinishedAt:   task.FinishedAt.Format("2006-01-02 15:04:05"),
		Status:       status,
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(notificationData)
	if err != nil {
		Logger.Errorf("序列化任务完成数据失败: %v", err)
		return fmt.Errorf("序列化任务完成数据失败: %v", err)
	}

	// 压缩并加密数据
	encryptedData, err := CompressAndEncrypt(jsonData)
	if err != nil {
		Logger.Errorf("加密任务完成数据失败: %v", err)
		return fmt.Errorf("加密任务完成数据失败: %v", err)
	}

	return sendNotification(encryptedData, "任务完成")
}

// NotifyTaskStepStart 通知任务步骤开始
func NotifyTaskStepStart(task *models.Task, step int, step_type string, stepName string) error {
	stepKey := fmt.Sprintf("step_%d_%s", step, step_type)

	// 获取上次该步骤的耗时
	lastDuration, err := GetLastStepDuration(task.Name, stepKey)
	if err != nil {
		Logger.Warnf("获取上次步骤耗时失败: %v", err)
		lastDuration = 0
	}

	// 预估当前步骤完成时间
	stepStartTime := time.Now()
	estimatedEnd := ""
	if estimatedEndTime, err := EstimateStepEndTime(task.Name, step, step_type, stepStartTime); err != nil {
		Logger.Warnf("预估步骤完成时间失败: %v", err)
	} else {
		estimatedEnd = estimatedEndTime.Format("2006-01-02 15:04:05")
	}

	// 构建步骤开始通知数据
	notificationData := UnifiedNotificationData{
		IsStep:        true,    // 步骤通知
		ID:            task.ID, // 任务ID
		Step:          step,
		StepType:      step_type,
		StepStartedAt: stepStartTime.Format("2006-01-02 15:04:05"),
		StepName:      stepName,
		StepStatus:    "running", // 开始执行
		Remote:        "server",
		LastDuration:  float64(int(lastDuration*100)) / 100, // 保留2位小数
		EstimatedEnd:  estimatedEnd,
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(notificationData)
	if err != nil {
		return fmt.Errorf("序列化步骤开始通知数据失败: %v", err)
	}

	// 压缩并加密数据
	encryptedData, err := CompressAndEncrypt(jsonData)
	if err != nil {
		return fmt.Errorf("加密步骤开始通知数据失败: %v", err)
	}

	//Logger.Infof("准备发送步骤%d开始通知: %s", step, stepName)
	return sendNotification(encryptedData, fmt.Sprintf("步骤%d开始", step))
}

// NotifyTaskStepCancel 通知任务步骤取消
func NotifyTaskStepCancel(task *models.Task, step int, step_type string, stepName string, stepStartTime, stepEndTime time.Time) error {
	stepDurationSeconds := stepEndTime.Sub(stepStartTime).Seconds()
	stepKey := fmt.Sprintf("step_%d_%s", step, step_type)

	// 获取上次该步骤的耗时
	lastDuration, err := GetLastStepDuration(task.Name, stepKey)
	if err != nil {
		Logger.Warnf("获取上次步骤耗时失败: %v", err)
		lastDuration = 0
	}

	// 预估任务结束时间
	estimatedEnd := ""
	if estimatedEndTime, err := EstimateStepEndTime(task.Name, step, step_type, stepStartTime); err != nil {
		Logger.Warnf("预估任务结束时间失败: %v", err)
	} else {
		estimatedEnd = estimatedEndTime.Format("2006-01-02 15:04:05")
	}

	// 构建步骤取消通知数据
	notificationData := UnifiedNotificationData{
		IsStep:         true,    // 步骤通知
		ID:             task.ID, // 任务ID
		Step:           step,
		StepType:       step_type,
		StepStartedAt:  stepStartTime.Format("2006-01-02 15:04:05"),
		StepFinishedAt: stepEndTime.Format("2006-01-02 15:04:05"),
		StepName:       stepName,
		StepStatus:     "cancel",                                    // 取消状态
		Duration:       float64(int(stepDurationSeconds*100)) / 100, // 保留2位小数
		Remote:         "server",
		LastDuration:   float64(int(lastDuration*100)) / 100, // 保留2位小数
		EstimatedEnd:   estimatedEnd,
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(notificationData)
	if err != nil {
		Logger.Errorf("序列化步骤取消通知数据失败: %v", err)
		return fmt.Errorf("序列化步骤取消通知数据失败: %v", err)
	}

	// 压缩并加密数据
	encryptedData, err := CompressAndEncrypt(jsonData)
	if err != nil {
		Logger.Errorf("加密步骤取消通知数据失败: %v", err)
		return fmt.Errorf("加密步骤取消通知数据失败: %v", err)
	}

	Logger.Infof("准备发送步骤%d取消通知: %s", step, stepName)
	return sendNotification(encryptedData, fmt.Sprintf("步骤%d取消", step))
}

// NotifyTaskStep 通知任务步骤完成
func NotifyTaskStep(task *models.Task, step int, step_type string, stepName string, stepStartTime, stepEndTime time.Time, success bool) error {
	stepDurationSeconds := stepEndTime.Sub(stepStartTime).Seconds()
	step_status := "success"
	if !success {
		step_status = "failed"
	}

	// 记录步骤耗时到任务的StepDurations中
	stepKey := fmt.Sprintf("step_%d_%s", step, step_type)
	if task.StepDurations != nil {
		task.StepDurations[stepKey] = stepDurationSeconds
	}

	// 获取上次该步骤的耗时（在保存新记录之前）
	lastDuration, err := GetLastStepDuration(task.Name, stepKey)
	if err != nil {
		Logger.Warnf("获取上次步骤耗时失败: %v", err)
		lastDuration = 0
	}

	// 保存步骤时间记录到隐藏文件（在获取上次时间之后）
	if err := AddStepTiming(task.Name, stepKey, stepName, stepDurationSeconds); err != nil {
		Logger.Warnf("保存步骤时间记录失败: %v", err)
	}

	// 预估任务结束时间
	estimatedEnd := ""
	if estimatedEndTime, err := EstimateStepEndTime(task.Name, step, step_type, stepStartTime); err != nil {
		Logger.Warnf("预估任务结束时间失败: %v", err)
	} else {
		estimatedEnd = estimatedEndTime.Format("2006-01-02 15:04:05")
	}

	// 构建步骤通知数据
	notificationData := UnifiedNotificationData{
		IsStep:         true,    // 步骤通知
		ID:             task.ID, // 任务ID
		Step:           step,
		StepType:       step_type,
		StepStartedAt:  stepStartTime.Format("2006-01-02 15:04:05"),
		StepFinishedAt: stepEndTime.Format("2006-01-02 15:04:05"),
		StepName:       stepName,
		StepStatus:     step_status,
		Duration:       float64(int(stepDurationSeconds*100)) / 100, // 保留2位小数
		Remote:         "server",
		LastDuration:   float64(int(lastDuration*100)) / 100, // 保留2位小数
		EstimatedEnd:   estimatedEnd,
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(notificationData)
	if err != nil {
		Logger.Errorf("序列化步骤通知数据失败: %v", err)
		return fmt.Errorf("序列化步骤通知数据失败: %v", err)
	}

	// 压缩并加密数据
	encryptedData, err := CompressAndEncrypt(jsonData)
	if err != nil {
		Logger.Errorf("加密步骤通知数据失败: %v", err)
		return fmt.Errorf("加密步骤通知数据失败: %v", err)
	}

	//Logger.Infof("准备发送步骤%d完成通知: %s (状态: %s)", step, stepName, step_status)
	return sendNotification(encryptedData, fmt.Sprintf("步骤%d完成", step))
}

// sendNotification 发送通知的通用函数
func sendNotification(encryptedData, notificationType string) error {
	// 构建请求体
	response := EncryptedResponse{
		Code: 200,
		Msg:  "success",
		Data: encryptedData,
	}

	// 序列化为JSON
	responseJson, err := json.Marshal(response)
	if err != nil {
		Logger.Errorf("序列化响应数据失败: %v", err)
		return fmt.Errorf("序列化响应数据失败: %v", err)
	}

	// 检查是否启用CMDB通知
	if !config.GetCMDBConfig().Enable {
		Logger.Info("CMDB通知功能已禁用，跳过通知")
		return nil
	}

	// 获取完整通知地址
	notifyURL := config.GetCMDBNotifyURL()
	if notifyURL == "" {
		Logger.Warn("CMDB通知地址未配置，跳过通知")
		return nil
	}

	// 发送HTTP请求
	resp, err := http.Post(notifyURL,
		"application/json", bytes.NewReader(responseJson))
	if err != nil {
		Logger.Errorf("发送通知请求失败: %v", err)
		return fmt.Errorf("发送通知请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		Logger.Errorf("读取响应失败: %v", err)
		return fmt.Errorf("读取响应失败: %v", err)
	}

	// 检查响应状态
	if resp.StatusCode != 200 {
		Logger.Errorf("远程接口返回错误: %s", string(respBody))
		return fmt.Errorf("远程接口返回错误: %s", string(respBody))
	}

	//Logger.Infof("%s通知发送成功", notificationType)
	return nil
}

// getTypeName 获取任务类型的中文名称
// web -> 前端, double/single -> 后端
func getTypeName(taskType string) string {
	switch taskType {
	case "web":
		return "前端"
	case "double", "single":
		return "后端"
	default:
		return "后端"
	}
}
