package config

import (
	"cicd-server/database"
	"cicd-server/models"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config 全局配置结构体
type Config struct {
	Logs         LogConfig           `mapstructure:"logs"`
	Database     DatabaseConfig      `mapstructure:"database"`
	CMDB         CMDBConfig          `mapstructure:"cmdb"`
	CICD         CICDConfig          `mapstructure:"cicd"`
	ProductRules map[string][]string `mapstructure:"product_rules"`
}

// 项目名 -> 产品类型的反向索引（启动时构建）
var projectTypeMap map[string]string

// LogConfig 日志配置结构体
type LogConfig struct {
	Enable bool   `mapstructure:"enable"` // 是否启用日志
	Level  string `mapstructure:"level"`  // 日志级别
}

// DatabaseConfig 数据库配置结构体
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`     // 数据库主机
	Port     int    `mapstructure:"port"`     // 数据库端口
	User     string `mapstructure:"user"`     // 数据库用户名
	Password string `mapstructure:"password"` // 数据库密码
	Database string `mapstructure:"database"` // 数据库名称
}

// CMDBConfig CMDB配置结构体
type CMDBConfig struct {
	BaseURL   string `mapstructure:"base_url"`   // CMDB服务地址（如 http://192.168.100.128:8080）
	NotifyURL string `mapstructure:"notify_url"` // 通知接口路径（如 /api/assets/proUpdate/records）
	UploadURL string `mapstructure:"upload_url"` // 文件上传接口路径（如 /api/upload/file）
	Enable    bool   `mapstructure:"enable"`     // 是否启用CMDB通知
}

// CICDConfig CICD配置结构体
type CICDConfig struct {
	EncryptionSalt string `mapstructure:"encryption_salt"` // 加密盐值
	Harbor         string `mapstructure:"harbor"`          // Harbor仓库地址
}

var (
	GlobalConfig   Config
	ProjectsConfig models.AppConfig
	viperInstance  *viper.Viper
)

// InitConfig 初始化配置
func InitConfig(configPath string) (*Config, error) {
	if configPath == "" {
		// 如果没有指定配置文件路径，使用默认路径
		workDir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("获取工作目录失败: %w", err)
		}
		configPath = filepath.Join(workDir, "config", "config.yml")
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := v.Unmarshal(&GlobalConfig); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 关键：全局保存viper实例
	viperInstance = v

	// 注意：项目配置需要在数据库初始化后加载，因此移到main.go中

	return &GlobalConfig, nil
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	return &GlobalConfig
}

// InitProjectsConfig 初始化项目配置（从MySQL加载）
func InitProjectsConfig(configPath string) error {
	// 从MySQL数据库加载项目配置
	projects, err := database.GetAllProjects()
	if err != nil {
		return fmt.Errorf("从数据库加载项目配置失败: %w", err)
	}

	// 填充到全局配置中
	ProjectsConfig.Projects = projects

	return nil
}

// GetProjectsConfig 获取项目配置
func GetProjectsConfig() *models.AppConfig {
	return &ProjectsConfig
}

// GetProjectConfig 获取指定项目的配置（从MySQL查询）
// 支持智能匹配：如果项目名带-web/-h5后缀，会先尝试精确匹配，失败则去掉后缀查询基础项目
func GetProjectConfig(projectName string) (*models.ProjectConfig, error) {
	// 优先从内存缓存获取
	if ProjectsConfig.Projects != nil {
		if project, exists := ProjectsConfig.Projects[projectName]; exists {
			return &project, nil
		}
	}

	// 如果内存中没有，从数据库查询
	project, err := database.GetProjectByName(projectName)
	if err != nil {
		// 如果查询失败，且项目名包含-web/-h5等前端标识，尝试去掉后缀查询
		baseProjectName := getBaseProjectName(projectName)
		if baseProjectName != projectName {
			// 尝试查询基础项目名
			project, err = database.GetProjectByName(baseProjectName)
			if err != nil {
				return nil, fmt.Errorf("项目配置未找到: %s (也尝试了 %s)", projectName, baseProjectName)
			}
			// 成功找到基础项目，使用基础项目名作为缓存key
			if ProjectsConfig.Projects == nil {
				ProjectsConfig.Projects = make(map[string]models.ProjectConfig)
			}
			ProjectsConfig.Projects[projectName] = *project     // 用原始名称缓存
			ProjectsConfig.Projects[baseProjectName] = *project // 用基础名称也缓存
			return project, nil
		}
		log.Printf("[ERROR] 项目配置未找到: %s", projectName)
		return nil, fmt.Errorf("项目配置未找到: %s", projectName)
	}

	log.Printf("[INFO] 成功查询到项目配置: %s", projectName)
	// 更新到内存缓存
	if ProjectsConfig.Projects == nil {
		ProjectsConfig.Projects = make(map[string]models.ProjectConfig)
	}
	ProjectsConfig.Projects[projectName] = *project

	return project, nil
}

// getBaseProjectName 获取基础项目名（去掉-web/-h5/-app等前端标识后缀）
func getBaseProjectName(projectName string) string {
	// 支持的前端标识后缀
	webSuffixes := []string{"-web", "-h5", "-app", "-admin", "-mobile"}

	for _, suffix := range webSuffixes {
		if strings.HasSuffix(projectName, suffix) {
			return strings.TrimSuffix(projectName, suffix)
		}
	}

	return projectName
}

// GetHarborDomain 获取Harbor域名
func GetHarborDomain() string {
	harbor := ""
	if GlobalConfig.CICD.Harbor != "" {
		harbor = GlobalConfig.CICD.Harbor
	} else {
		harbor = "hub.hzbxhd.com" // 默认值
	}

	// 去除协议前缀（如果存在）
	if strings.HasPrefix(harbor, "https://") {
		harbor = strings.TrimPrefix(harbor, "https://")
	} else if strings.HasPrefix(harbor, "http://") {
		harbor = strings.TrimPrefix(harbor, "http://")
	}

	return harbor
}

// GetEncryptionSalt 获取加密盐
func GetEncryptionSalt() string {
	if viperInstance == nil {
		return ""
	}
	salt := viperInstance.GetString("cicd.encryption_salt")
	return salt
}

// GetCMDBConfig 获取CMDB配置
func GetCMDBConfig() *CMDBConfig {
	return &GlobalConfig.CMDB
}

// GetCMDBUploadURL 获取CMDB文件上传完整地址
func GetCMDBUploadURL() string {
	cfg := GlobalConfig.CMDB
	if cfg.UploadURL == "" {
		return ""
	}
	return cfg.BaseURL + cfg.UploadURL
}

// GetCMDBNotifyURL 获取CMDB通知完整地址
func GetCMDBNotifyURL() string {
	cfg := GlobalConfig.CMDB
	if cfg.NotifyURL == "" {
		return ""
	}
	return cfg.BaseURL + cfg.NotifyURL
}

// InitProductRules 初始化产品规则配置（支持热更新）
// 产品规则在 config.yml 的 product_rules 字段中配置
func InitProductRules() error {
	if viperInstance == nil {
		return fmt.Errorf("viper 实例未初始化，请先调用 InitConfig")
	}

	// 构建反向索引
	buildProjectTypeMap()

	// 热更新监听
	viperInstance.WatchConfig()
	viperInstance.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("[INFO] 配置文件变更: %s", e.Name)
		var newConfig Config
		if err := viperInstance.Unmarshal(&newConfig); err != nil {
			log.Printf("[ERROR] 重新解析配置失败: %v", err)
			return
		}
		GlobalConfig = newConfig
		buildProjectTypeMap()
		// 热更新日志由 main.go 通过回调输出
		if onProductRulesChange != nil {
			onProductRulesChange()
		}
	})

	return nil
}

// onProductRulesChange 热更新回调（由 main.go 设置）
var onProductRulesChange func()

// SetOnProductRulesChange 设置产品规则热更新回调
func SetOnProductRulesChange(fn func()) {
	onProductRulesChange = fn
}

// GetProductRules 获取产品规则配置
func GetProductRules() map[string][]string {
	return GlobalConfig.ProductRules
}

// GetProjectTypeMap 获取项目名→产品类型的反向索引
func GetProjectTypeMap() map[string]string {
	return projectTypeMap
}

// buildProjectTypeMap 从 ProductRules 构建项目名→产品类型的反向索引
func buildProjectTypeMap() {
	projectTypeMap = make(map[string]string)
	for productType, projects := range GlobalConfig.ProductRules {
		for _, project := range projects {
			projectTypeMap[project] = productType
		}
	}
}

// GetProductType 根据项目名获取产品类型
func GetProductType(projectName string) (string, bool) {
	if projectTypeMap == nil {
		return "", false
	}
	productType, exists := projectTypeMap[projectName]
	return productType, exists
}
