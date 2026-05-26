package common

import (
	"bytes"
	"cicd-server/models"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CallbackData 回调数据结构
type CallbackData struct {
	Project       string             `json:"project"`                // 项目名称(非中文)
	ProjectName   string             `json:"project_name"`           // 项目名称(中文)
	Category      string             `json:"category"`               // 额外参数
	Description   string             `json:"description"`            // 项目描述
	Status        string             `json:"status"`                 // 任务状态
	Tag           string             `json:"tag"`                    // 镜像tag
	Type          string             `json:"type"`                   // 任务类型（web/double/single）
	CreateTime    string             `json:"create_time"`            // 创建时间
	UpdateFeishu  string             `json:"update_feishu"`          // 发版通知地址（替代ops飞书）
	NotifyFeishu  string             `json:"notify_feishu"`          // 普通通知地址（替代pro飞书）
	StepDurations map[string]float64 `json:"step_durations"`         // 每一步的耗时（秒）
	TaskID        string             `json:"task_id"`                // 任务ID
	DownloadURL   string             `json:"download_url,omitempty"` // 构建产物下载地址
}

// SendTaskCallback 发送任务完成回调
func SendTaskCallback(task *models.Task) error {
	if task.CallbackURL == "" {
		return nil // 没有回调地址，直接返回
	}

	//Logger.Infof("开始发送任务回调: %s -> %s", task.ID, task.CallbackURL)

	// 获取描述：优先使用请求传入的描述，否则使用项目配置中的描述
	description := task.Description
	if description == "" && task.ProjectInfo != nil {
		description = task.ProjectInfo.Description
	}

	// 构造回调数据
	callbackData := &CallbackData{
		Project:     task.Name,
		ProjectName: task.ProjectInfo.ProjectName,
		Description: description,
		CreateTime:  task.CreatedAt.Format("2006-01-02 15:04:05"),
		Status:      string(task.Status),
		Tag:         task.ImageTag,
		Type:        task.Type,
		TaskID:      task.ID,
	}

	// 设置额外参数
	if task.Category != "" {
		callbackData.Category = task.Category
	}
	// 设置飞书地址
	if task.ProjectInfo != nil {
		callbackData.UpdateFeishu = task.ProjectInfo.UpdateFeishu
		callbackData.NotifyFeishu = task.ProjectInfo.NotifyFeishu
	}

	// 设置步骤耗时
	callbackData.StepDurations = task.StepDurations

	// 设置产物下载地址（Web项目）
	if task.DownloadURL != "" {
		callbackData.DownloadURL = task.DownloadURL
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(callbackData)
	if err != nil {
		Logger.Errorf("序列化回调数据失败: %v", err)
		return fmt.Errorf("序列化回调数据失败: %v", err)
	}

	Logger.Infof("回调数据: %s", string(jsonData))

	// 加密回调数据
	encryptedData, err := CompressAndEncrypt(jsonData)
	if err != nil {
		Logger.Errorf("加密回调数据失败: %v", err)
		return fmt.Errorf("加密回调数据失败: %v", err)
	}

	// 构建加密请求体
	encryptedRequest := map[string]string{
		"data": encryptedData,
	}

	encryptedJsonData, err := json.Marshal(encryptedRequest)
	if err != nil {
		Logger.Errorf("序列化加密请求失败: %v", err)
		return fmt.Errorf("序列化加密请求失败: %v", err)
	}

	// 发送HTTP POST请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Post(task.CallbackURL, "application/json", bytes.NewBuffer(encryptedJsonData))
	if err != nil {
		Logger.Errorf("发送回调请求失败: %v", err)
		return fmt.Errorf("发送回调请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		Logger.Infof("任务回调发送成功: %s, 状态码: %d", task.ID, resp.StatusCode)
		return nil
	} else {
		Logger.Errorf("任务回调发送失败: %s, 状态码: %d", task.ID, resp.StatusCode)
		return fmt.Errorf("回调服务返回错误状态码: %d", resp.StatusCode)
	}
}
