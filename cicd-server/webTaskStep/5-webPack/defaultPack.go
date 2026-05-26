package webPack

import (
	"cicd-server/models"
	"fmt"
	"path/filepath"
	"time"
)

// ExecuteWebPack 执行Web项目打包
func ExecuteWebPack(task *models.Task, nodeVersion, gitCloneDir, logDir, timeDir, taskLogDir string,
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

	addTaskLog(task, "开始Web项目打包")

	// 获取Node镜像
	nodeImage := getNodeImage(nodeVersion)
	addTaskLog(task, fmt.Sprintf("使用Node镜像: %s", nodeImage))

	// 创建npm build命令，根据category动态选择工作目录
	buildLogFile := filepath.Join(taskLogDir, "build.log")
	var workDir string
	if task.Category != "" {
		workDir = fmt.Sprintf("/app/%s", task.Category)
	} else {
		workDir = "/app"
	}
	var buildCmd string
	if task.Name == "scfq" {
		buildCmd = fmt.Sprintf("docker run --rm -v %s:/app %s /bin/sh -c \"cd %s && npm run build\"",
			gitCloneDir, nodeImage, workDir)
	} else {
		buildCmd = fmt.Sprintf("docker run --rm -v %s:/app %s /bin/sh -c \"cd %s && npm run build:prod\"",
			gitCloneDir, nodeImage, workDir)
	}

	addTaskLog(task, fmt.Sprintf("执行命令: %s", buildCmd))
	addTaskLog(task, fmt.Sprintf("构建日志输出到: %s", buildLogFile))

	buildStartTime := time.Now()

	if err := executeCommandWithLog(task, buildCmd, buildLogFile); err != nil {
		addTaskLog(task, fmt.Sprintf("Web项目打包失败: %v", err))
		return err
	}

	buildEndTime := time.Now()
	buildDuration := buildEndTime.Sub(buildStartTime).Seconds()
	addTaskLog(task, fmt.Sprintf("Web项目打包完成，耗时: %.2f秒", buildDuration))

	// 记录构建时间
	if err := appendToFile(filepath.Join(timeDir, "ready_time.txt"),
		fmt.Sprintf("%.0f", buildDuration)); err != nil {
		addTaskLog(task, fmt.Sprintf("写入构建时间失败: %v", err))
	}

	return nil
}

// getNodeImage 根据Node版本获取对应的Docker镜像
func getNodeImage(nodeVersion string) string {
	return "prohub.hzbxhd.com/library/npm-zh:1.0"
}
