package common

// file_logger.go 已废弃
//
// 说明：由于日志架构简化，不再使用任务总日志文件（logs/tasks/{taskID}.log）
// 现在日志分为两个输出位置：
// 1. 控制台 - 服务运行日志（使用 common.Logger）
// 2. 工作日志目录 - 任务执行步骤日志（/data/workspace/{projectName}/logs/{taskID}/{stepType}.log）
//
// 原有的 FileLogger 及相关方法已全部移除

// SplitLines 按行分割文本（保留的工具函数）
func SplitLines(text string) []string {
	var lines []string
	var line string

	for _, r := range text {
		if r == '\n' {
			lines = append(lines, line)
			line = ""
		} else {
			line += string(r)
		}
	}

	if line != "" {
		lines = append(lines, line)
	}

	return lines
}
