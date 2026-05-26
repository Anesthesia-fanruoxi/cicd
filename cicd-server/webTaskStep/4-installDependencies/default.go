package installDependencies

import (
	"cicd-server/models"
	"fmt"
	"path/filepath"
	"time"
)

// ExecuteInstallDependencies 执行Node.js依赖安装
func ExecuteInstallDependencies(task *models.Task, nodeVersion, gitCloneDir, logDir, timeDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error,
	executeCommandWithLog func(*models.Task, string, string) error,
	appendToFile func(string, string) error) error {

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	addTaskLog(task, "开始安装Node.js依赖")

	// 获取Node镜像
	nodeImage := getNodeImage(nodeVersion)
	addTaskLog(task, fmt.Sprintf("使用Node镜像: %s", nodeImage))

	// 创建npm安装命令，根据category动态选择工作目录
	npmLogFile := filepath.Join(taskLogDir, "npm.log")
	var workDir string
	if task.Category != "" {
		workDir = fmt.Sprintf("/app/%s", task.Category)
	} else {
		workDir = "/app"
	}
	npmCmd := fmt.Sprintf("docker run --rm -v %s:/app %s /bin/sh -c \"cd %s && npm install\"",
		gitCloneDir, nodeImage, workDir)

	addTaskLog(task, fmt.Sprintf("执行命令: %s", npmCmd))
	addTaskLog(task, fmt.Sprintf("npm日志输出到: %s", npmLogFile))

	npmStartTime := time.Now()

	if err := executeCommandWithLog(task, npmCmd, npmLogFile); err != nil {
		addTaskLog(task, fmt.Sprintf("npm install失败: %v", err))
		return err
	}

	npmEndTime := time.Now()
	npmDuration := npmEndTime.Sub(npmStartTime).Seconds()
	addTaskLog(task, fmt.Sprintf("依赖安装完成，耗时: %.2f秒", npmDuration))

	// 记录npm安装时间
	if err := appendToFile(filepath.Join(timeDir, "ready_time.txt"),
		fmt.Sprintf("%.0f", npmDuration)); err != nil {
		addTaskLog(task, fmt.Sprintf("写入npm安装时间失败: %v", err))
	}

	return nil
}

// getNodeImage 根据Node版本获取对应的Docker镜像
func getNodeImage(nodeVersion string) string {
	return "prohub.hzbxhd.com/library/npm-zh:1.0"
}
