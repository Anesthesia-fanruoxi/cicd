package common

import (
	"bytes"
	"cicd-server/models"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// FeishuCardMessage 飞书卡片消息结构
type FeishuCardMessage struct {
	MsgType string     `json:"msg_type"`
	Card    FeishuCard `json:"card"`
}

// FeishuCard 飞书卡片结构
type FeishuCard struct {
	Config   FeishuCardConfig `json:"config"`
	Elements []FeishuElement  `json:"elements"`
	Header   FeishuCardHeader `json:"header"`
}

// FeishuCardConfig 卡片配置
type FeishuCardConfig struct {
	WideScreenMode bool `json:"wide_screen_mode"`
}

// FeishuCardHeader 卡片头部
type FeishuCardHeader struct {
	Title    FeishuText `json:"title"`
	Template string     `json:"template"`
}

// FeishuElement 卡片元素接口
type FeishuElement interface{}

// FeishuText 文本结构
type FeishuText struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

// FeishuDivElement 分割线元素
type FeishuDivElement struct {
	Tag    string               `json:"tag"`
	Fields []FeishuFieldElement `json:"fields,omitempty"`
	Text   *FeishuText          `json:"text,omitempty"`
}

// FeishuFieldElement 字段元素
type FeishuFieldElement struct {
	IsShort bool       `json:"is_short"`
	Text    FeishuText `json:"text"`
}

// FeishuHrElement 分割线元素
type FeishuHrElement struct {
	Tag string `json:"tag"`
}

// FeishuActionElement 操作元素
type FeishuActionElement struct {
	Tag     string                `json:"tag"`
	Actions []FeishuButtonElement `json:"actions"`
}

// FeishuButtonElement 按钮元素
type FeishuButtonElement struct {
	Tag  string     `json:"tag"`
	Text FeishuText `json:"text"`
	URL  string     `json:"url,omitempty"`
	Type string     `json:"type,omitempty"`
}

// SendFeishuTaskCreateNotification 发送任务创建飞书通知
func SendFeishuTaskCreateNotification(task *models.Task) error {
	if task == nil || task.ProjectInfo == nil {
		return fmt.Errorf("任务或项目信息为空")
	}

	// 检查是否有运维飞书URL
	if task.ProjectInfo.UpdateFeishu == "" {
		Logger.Warn("项目运维飞书URL为空，跳过飞书通知")
		return nil
	}

	// 构建飞书卡片消息
	card := buildTaskCreateCard(task)

	// 发送飞书消息
	return sendFeishuMessage(task.ProjectInfo.UpdateFeishu, card)
}

// SendFeishuTaskCompleteNotification 发送任务完成飞书通知
func SendFeishuTaskCompleteNotification(task *models.Task) error {
	if task == nil || task.ProjectInfo == nil {
		return fmt.Errorf("任务或项目信息为空")
	}

	// 检查是否有运维飞书URL
	if task.ProjectInfo.UpdateFeishu == "" {
		Logger.Warn("项目运维飞书URL为空，跳过飞书通知")
		return nil
	}

	// 构建飞书卡片消息
	card := buildTaskCompleteCard(task)

	// 发送飞书消息
	return sendFeishuMessage(task.ProjectInfo.UpdateFeishu, card)
}

// buildTaskCreateCard 构建任务创建卡片
func buildTaskCreateCard(task *models.Task) FeishuCardMessage {
	// 确定卡片颜色和状态
	template := "blue"
	// 获取任务类型中文名称
	typeName := getTypeName(task.Type)
	// 构建状态文本，如果有项目描述则显示
	var statusText string
	if task.ProjectInfo != nil && task.ProjectInfo.ProjectName != "" {
		statusText = fmt.Sprintf("🚀 【%s-%s】任务已创建", task.ProjectInfo.ProjectName, typeName)
	} else {
		statusText = "🚀 任务已创建"
	}

	// 格式化时间
	createdTime := task.CreatedAt.Format("2006-01-02 15:04:05")

	// 构建基础字段（四个字段两两布局）
	baseFields := []FeishuFieldElement{
		{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**项目名称**\n%s", task.Name),
			},
		},
		{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**任务类型**\n%s", typeName),
			},
		},
		{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**任务ID**\n%s", task.ID),
			},
		},
		{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**创建时间**\n%s", createdTime),
			},
		},
	}

	// 构建卡片元素
	elements := []FeishuElement{
		FeishuDivElement{
			Tag:    "div",
			Fields: baseFields,
		},
	}

	// 添加分割线
	elements = append(elements, FeishuHrElement{Tag: "hr"})

	// 添加额外参数（如果存在）
	if task.Category != "" {
		elements = append(elements, FeishuDivElement{
			Tag: "div",
			Text: &FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**额外参数**\n%s", task.Category),
			},
		})
	}

	return FeishuCardMessage{
		MsgType: "interactive",
		Card: FeishuCard{
			Config: FeishuCardConfig{
				WideScreenMode: true,
			},
			Header: FeishuCardHeader{
				Title: FeishuText{
					Tag:     "plain_text",
					Content: statusText,
				},
				Template: template,
			},
			Elements: elements,
		},
	}
}

// buildTaskCompleteCard 构建任务完成卡片
func buildTaskCompleteCard(task *models.Task) FeishuCardMessage {
	// 获取任务类型中文名称
	typeName := getTypeName(task.Type)
	// 确定卡片颜色和状态
	var template, statusText, statusIcon string
	switch task.Status {
	case models.TaskStatusFailed:
		template = "red"
		statusText = fmt.Sprintf("❌ 【%s-%s】任务执行失败", task.ProjectInfo.ProjectName, typeName)
		statusIcon = "❌"
	case models.TaskStatusCanceled:
		template = "grey"
		statusText = fmt.Sprintf("⏹️ 【%s-%s】任务已取消", task.ProjectInfo.ProjectName, typeName)
		statusIcon = "⏹️"
	default:
		// 其他状态（理论上不应该到这里，因为成功状态不会发送飞书通知）
		template = "blue"
		statusText = fmt.Sprintf("📋 【%s-%s】任务状态异常", task.ProjectInfo.ProjectName, typeName)
		statusIcon = "📋"
	}

	// 格式化时间
	createdTime := task.CreatedAt.Format("2006-01-02 15:04:05")
	var startedTime, finishedTime string
	if task.StartedAt != nil {
		startedTime = task.StartedAt.Format("2006-01-02 15:04:05")
	}
	if task.FinishedAt != nil {
		finishedTime = task.FinishedAt.Format("2006-01-02 15:04:05")
	}

	// 计算执行时长
	var duration string
	if task.StartedAt != nil && task.FinishedAt != nil {
		d := task.FinishedAt.Sub(*task.StartedAt)
		totalSeconds := int(d.Seconds())

		if totalSeconds >= 3600 { // 超过1小时
			hours := totalSeconds / 3600
			minutes := (totalSeconds % 3600) / 60
			seconds := totalSeconds % 60
			if minutes > 0 && seconds > 0 {
				duration = fmt.Sprintf("%d小时%d分%d秒", hours, minutes, seconds)
			} else if minutes > 0 {
				duration = fmt.Sprintf("%d小时%d分", hours, minutes)
			} else {
				duration = fmt.Sprintf("%d小时", hours)
			}
		} else if totalSeconds >= 60 { // 1分钟到1小时
			minutes := totalSeconds / 60
			seconds := totalSeconds % 60
			if seconds > 0 {
				duration = fmt.Sprintf("%d分%d秒", minutes, seconds)
			} else {
				duration = fmt.Sprintf("%d分", minutes)
			}
		} else { // 小于1分钟
			duration = fmt.Sprintf("%d秒", totalSeconds)
		}
	}

	// 构建基础字段（包含结束相关信息）
	baseFields := []FeishuFieldElement{
		{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**项目名称**\n%s", task.Name),
			},
		},
		{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**任务类型**\n%s", typeName),
			},
		},
		{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**任务ID**\n%s", task.ID),
			},
		},
		{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**创建时间**\n%s", createdTime),
			},
		},
	}

	// 添加结束相关信息到基础字段区域
	if startedTime != "" {
		baseFields = append(baseFields, FeishuFieldElement{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**开始时间**\n%s", startedTime),
			},
		})
	}

	if finishedTime != "" {
		baseFields = append(baseFields, FeishuFieldElement{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**结束时间**\n%s", finishedTime),
			},
		})
	}

	if duration != "" {
		baseFields = append(baseFields, FeishuFieldElement{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**执行时长**\n%s", duration),
			},
		})
	}

	// 添加任务状态
	baseFields = append(baseFields, FeishuFieldElement{
		IsShort: true,
		Text: FeishuText{
			Tag:     "lark_md",
			Content: fmt.Sprintf("**任务状态**\n%s %s", statusIcon, string(task.Status)),
		},
	})

	// 添加执行结果（如果有）
	if task.Result != "" {
		baseFields = append(baseFields, FeishuFieldElement{
			IsShort: true,
			Text: FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**执行结果**\n%s", task.Result),
			},
		})
	}

	// 构建卡片元素
	elements := []FeishuElement{
		FeishuDivElement{
			Tag:    "div",
			Fields: baseFields,
		},
	}

	// 添加分割线
	elements = append(elements, FeishuHrElement{Tag: "hr"})

	// 只添加额外参数（如果存在）
	if task.Category != "" {
		elements = append(elements, FeishuDivElement{
			Tag: "div",
			Text: &FeishuText{
				Tag:     "lark_md",
				Content: fmt.Sprintf("**额外参数**\n%s", task.Category),
			},
		})
	}

	return FeishuCardMessage{
		MsgType: "interactive",
		Card: FeishuCard{
			Config: FeishuCardConfig{
				WideScreenMode: true,
			},
			Header: FeishuCardHeader{
				Title: FeishuText{
					Tag:     "plain_text",
					Content: statusText,
				},
				Template: template,
			},
			Elements: elements,
		},
	}
}

// sendFeishuMessage 发送飞书消息
func sendFeishuMessage(webhookURL string, message FeishuCardMessage) error {
	// 序列化消息
	jsonData, err := json.Marshal(message)
	if err != nil {
		Logger.Errorf("序列化飞书消息失败: %v", err)
		return fmt.Errorf("序列化飞书消息失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		Logger.Errorf("创建飞书请求失败: %v", err)
		return fmt.Errorf("创建飞书请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 创建HTTP客户端并发送请求
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		Logger.Errorf("发送飞书消息失败: %v", err)
		return fmt.Errorf("发送飞书消息失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != 200 {
		Logger.Errorf("飞书接口返回错误状态码: %d", resp.StatusCode)
		return fmt.Errorf("飞书接口返回错误状态码: %d", resp.StatusCode)
	}

	//Logger.Info("飞书通知发送成功")
	return nil
}
