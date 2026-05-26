package models

// ProjectConfig 项目配置结构体（对齐CMDB的sys_project_dict表）
type ProjectConfig struct {
	ProjectName      string `json:"project_name"`      // 项目中文名
	GitVue           string `json:"git_vue"`           // 前端git仓库
	GitBackend       string `json:"git_backend"`       // 后端git仓库
	UpdateFeishu     string `json:"update_feishu"`     // 发版通知地址（替代ops飞书）
	NotifyFeishu     string `json:"notify_feishu"`     // 普通通知地址（替代pro飞书）
	Description      string `json:"description"`       // 项目描述
	BackendTool      string `json:"backend_tool"`      // 后端工具（java17/java21等）
	FrontendTool     string `json:"frontend_tool"`     // 前端工具（node14/node16等）
	EnableSkyWalking bool   `json:"enable_skywalking"` // 是否启用skywalking
	CreatedBy        int64  `json:"created_by"`        // 创建人ID
}

// SpecialLists 特殊项目列表
type SpecialLists struct {
	MonomerList []string `json:"monomer_list"`
	OnlyList    []string `json:"only_list"`
}

// AppConfig 应用配置结构体
type AppConfig struct {
	Projects     map[string]ProjectConfig `json:"projects"`
	SpecialLists SpecialLists             `json:"special_lists"`
}
