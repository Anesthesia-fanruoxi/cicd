package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StepTimingRecord 步骤时间记录
type StepTimingRecord struct {
	StepKey   string  `json:"step_key"`  // 步骤标识 (step_1_init, step_2_clear等)
	StepName  string  `json:"step_name"` // 步骤名称
	Duration  float64 `json:"duration"`  // 耗时(秒)
	Timestamp string  `json:"timestamp"` // 记录时间
}

// ProjectTimingHistory 项目时间历史记录
type ProjectTimingHistory struct {
	ProjectName string             `json:"project_name"`
	Records     []StepTimingRecord `json:"records"`
}

// getTimingFilePath 获取项目的时间记录文件路径
func getTimingFilePath(projectName string) string {
	// 在工作空间目录下创建隐藏文件
	workspaceDir := fmt.Sprintf("/data/workspace/%s", projectName)
	return filepath.Join(workspaceDir, ".step_times.json")
}

// LoadProjectTimingHistory 加载项目时间历史记录
func LoadProjectTimingHistory(projectName string) (*ProjectTimingHistory, error) {
	filePath := getTimingFilePath(projectName)

	// 如果文件不存在，返回空记录
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return &ProjectTimingHistory{
			ProjectName: projectName,
			Records:     []StepTimingRecord{},
		}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取时间记录文件失败: %v", err)
	}

	var history ProjectTimingHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("解析时间记录文件失败: %v", err)
	}

	return &history, nil
}

// SaveProjectTimingHistory 保存项目时间历史记录
func SaveProjectTimingHistory(history *ProjectTimingHistory) error {
	filePath := getTimingFilePath(history.ProjectName)

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化时间记录失败: %v", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入时间记录文件失败: %v", err)
	}

	return nil
}

// AddStepTiming 添加步骤时间记录
func AddStepTiming(projectName, stepKey, stepName string, duration float64) error {
	history, err := LoadProjectTimingHistory(projectName)
	if err != nil {
		Logger.Errorf("加载项目时间历史失败: %v", err)
		return err
	}

	// 添加新记录
	record := StepTimingRecord{
		StepKey:   stepKey,
		StepName:  stepName,
		Duration:  duration,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}

	history.Records = append(history.Records, record)

	// 保持最近50条记录，避免文件过大
	if len(history.Records) > 50 {
		history.Records = history.Records[len(history.Records)-50:]
	}

	return SaveProjectTimingHistory(history)
}

// GetLastStepDuration 获取上次该步骤的耗时
func GetLastStepDuration(projectName, stepKey string) (float64, error) {
	history, err := LoadProjectTimingHistory(projectName)
	if err != nil {
		return 0, err
	}

	// 从后往前查找最近的该步骤记录
	for i := len(history.Records) - 1; i >= 0; i-- {
		if history.Records[i].StepKey == stepKey {
			return history.Records[i].Duration, nil
		}
	}

	return 0, nil // 没有找到历史记录
}

// GetAverageStepDuration 获取该步骤的平均耗时
func GetAverageStepDuration(projectName, stepKey string) (float64, error) {
	history, err := LoadProjectTimingHistory(projectName)
	if err != nil {
		return 0, err
	}

	var total float64
	var count int

	for _, record := range history.Records {
		if record.StepKey == stepKey {
			total += record.Duration
			count++
		}
	}

	if count == 0 {
		return 0, nil // 没有历史记录
	}

	return total / float64(count), nil
}

// EstimateStepEndTime 预估当前步骤完成时间
func EstimateStepEndTime(projectName string, currentStep int, currentStepType string, stepStartTime time.Time) (time.Time, error) {
	// 获取当前步骤的预估耗时
	currentStepKey := fmt.Sprintf("step_%d_%s", currentStep, currentStepType)
	currentStepDuration, err := GetLastStepDuration(projectName, currentStepKey)
	if err != nil || currentStepDuration == 0 {
		currentStepDuration = getDefaultStepDuration(currentStepKey)
	}

	// 当前步骤预估完成时间 = 开始时间 + 上次耗时
	return stepStartTime.Add(time.Duration(currentStepDuration * float64(time.Second))), nil
}

// getDefaultStepDuration 获取步骤的默认预估时间(秒)
func getDefaultStepDuration(stepKey string) float64 {
	defaults := map[string]float64{
		"step_1_init":       5,  // 初始化
		"step_2_clear":      3,  // 清理
		"step_3_git":        15, // Git克隆
		"step_4_mvn":        60, // Maven编译
		"step_4_npm":        30, // NPM编译
		"step_5_dockerfile": 5,  // 创建Dockerfile
		"step_6_build":      45, // 构建镜像
		"step_7_push":       20, // 推送镜像
		"step_8_upload":     10, // 上传产物
	}

	if duration, exists := defaults[stepKey]; exists {
		return duration
	}

	return 30 // 默认30秒
}
