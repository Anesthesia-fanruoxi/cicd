package models

import (
	"math/rand"
	"sync"
	"time"
)

// IsCrmProject 判断是否为CRM项目类型
func IsCrmProject(projectName string) bool {
	return projectName == "bjjf-crm" || projectName == "axh-crm"
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Project       string `json:"project"`         // 项目名称
	Type          string `json:"type"`            // 任务类型（web/double/single等）
	CallbackURL   string `json:"callback_url"`    // 回调地址
	Category      string `json:"category"`        // 项目分类（用于多子项目）
	Description   string `json:"description"`     // 任务描述（可选，覆盖项目配置中的描述）
	CreatedBy     int    `json:"created_by"`      // 创建人ID
	CreatedByName string `json:"created_by_name"` // 创建人名称
}

// TaskListItem 任务列表项
type TaskListItem struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"` // 任务类型（web/double/single）
	Status     TaskStatus `json:"status"`
	CreatedAt  string     `json:"createdAt"`
	StartedAt  string     `json:"startedAt,omitempty"`
	FinishedAt string     `json:"finishedAt,omitempty"`
}

// TaskDetail 任务详情
type TaskDetail struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"` // 任务类型（web/double/single）
	Status     TaskStatus `json:"status"`
	CreatedAt  string     `json:"createdAt"`
	StartedAt  string     `json:"startedAt,omitempty"`
	FinishedAt string     `json:"finishedAt,omitempty"`
	Result     string     `json:"result"`
	// 项目信息字段（扁平化）
	GitURL       string `json:"git_url,omitempty"`
	Description  string `json:"description,omitempty"`
	UpdateFeishu string `json:"update_feishu,omitempty"` // 发版通知地址（替代ops飞书）
	NotifyFeishu string `json:"notify_feishu,omitempty"` // 普通通知地址（替代pro飞书）
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"  // 等待中
	TaskStatusRunning  TaskStatus = "running"  // 执行中
	TaskStatusSuccess  TaskStatus = "success"  // 执行成功
	TaskStatusFailed   TaskStatus = "failed"   // 执行失败
	TaskStatusCanceled TaskStatus = "canceled" // 已取消
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeDefault TaskType = "default" // 默认任务类型
	TaskTypeWeb     TaskType = "web"     // Web任务类型
	TaskTypeCrm     TaskType = "crm"     // CRM任务类型
)

// Task 任务结构体
type Task struct {
	ID            string             // 任务ID
	Name          string             // 任务名称
	Type          string             // 任务类型（web/double/single）
	Status        TaskStatus         // 任务状态
	CreatedAt     time.Time          // 创建时间
	StartedAt     *time.Time         // 开始执行时间
	FinishedAt    *time.Time         // 完成时间
	Result        string             // 任务结果
	LogDir        string             // 工作目录日志路径
	CancelChan    chan struct{}      // 取消信号
	CancelFunc    func()             // 取消函数
	ProjectInfo   *ProjectConfig     // 项目信息
	CallbackURL   string             // 回调地址
	StepDurations map[string]float64 // 每步的耗时（秒）
	ImageTag      string             // 镜像标签
	Category      string             // 项目分类（用于多子项目）
	Description   string             // 任务描述（请求传入，覆盖项目配置）
	DownloadURL   string             // 构建产物下载地址（Web项目）
	CreatedBy     int                // 创建人ID
	CreatedByName string             // 创建人名称
}

// WorkerPool 工作线程池
type WorkerPool struct {
	Queue       []*Task // 等待队列
	MaxWorkers  int     // 最大工作线程数
	WorkerCount int     // 当前工作线程数
}

// TaskManager 任务管理器
type TaskManager struct {
	Tasks       map[string]*Task         // 所有任务
	WorkerPools map[TaskType]*WorkerPool // 按类型的工作线程池
	Mutex       sync.Mutex               // 锁
	Random      *rand.Rand               // 随机数生成器
}
