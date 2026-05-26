package buildImage

import (
	"cicd-server/models"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

// ExecuteBuildImage 执行Docker镜像构建
func ExecuteBuildImage(task *models.Task, projectName, imageDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error,
	executeCommandWithLog func(*models.Task, string, string) error) error {

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	addTaskLog(task, "开始构建Docker镜像")

	// 构建镜像（使用docker-compose并行构建）
	if err := buildDockerImages(task, imageDir, taskLogDir, addTaskLog, executeCommandWithLog); err != nil {
		return fmt.Errorf("构建Docker镜像失败: %v", err)
	}

	addTaskLog(task, "Docker镜像构建完成")
	return nil
}

// buildDockerImages 使用docker-compose构建镜像
func buildDockerImages(task *models.Task, imageDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommandWithLog func(*models.Task, string, string) error) error {

	addTaskLog(task, "开始执行docker-compose构建")

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	// 构建命令：cd $image_dir && docker-compose -f docker-compose.yaml build --pull
	buildStartTime := time.Now()
	buildLogFile := filepath.Join(taskLogDir, "build.log")

	// 使用docker-compose进行并行构建
	buildCmd := fmt.Sprintf("cd %s && docker compose -f docker-compose.yaml build --pull", imageDir)

	addTaskLog(task, fmt.Sprintf("执行构建命令: %s", buildCmd))
	addTaskLog(task, fmt.Sprintf("构建日志输出到: %s", buildLogFile))

	if err := executeCommandWithLog(task, buildCmd, buildLogFile); err != nil {
		addTaskLog(task, fmt.Sprintf("Docker镜像构建失败: %v", err))

		// 提取构建错误日志
		if err := extractBuildErrors(task, buildLogFile, taskLogDir, addTaskLog); err != nil {
			addTaskLog(task, fmt.Sprintf("提取构建错误日志失败: %v", err))
		}

		return err
	}

	buildEndTime := time.Now()
	buildDuration := buildEndTime.Sub(buildStartTime).Seconds()
	addTaskLog(task, fmt.Sprintf("Docker镜像构建完成，耗时: %.2f秒", buildDuration))

	// 验证构建结果
	if err := verifyBuildSuccess(task, imageDir, addTaskLog); err != nil {
		addTaskLog(task, fmt.Sprintf("构建结果验证失败: %v", err))
		return err
	}

	return nil
}

// extractBuildErrors 提取构建错误日志
func extractBuildErrors(task *models.Task, buildLogFile, taskLogDir string,
	addTaskLog func(*models.Task, string)) error {

	buildErrorFile := filepath.Join(taskLogDir, "build_error.log")

	// 提取ERROR关键字的日志行
	grepCmd := fmt.Sprintf("grep -i \"ERROR\\|FAILED\\|FAILURE\" %s | uniq > %s", buildLogFile, buildErrorFile)

	cmd := exec.Command("bash", "-c", grepCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行grep命令失败: %v", err)
	}

	addTaskLog(task, fmt.Sprintf("构建错误日志已提取到: %s", buildErrorFile))
	return nil
}

// verifyBuildSuccess 验证构建是否成功
func verifyBuildSuccess(task *models.Task, imageDir string,
	addTaskLog func(*models.Task, string)) error {

	addTaskLog(task, "开始验证构建结果")

	// 读取images.txt文件获取应该构建的镜像列表
	imagesFile := filepath.Join(imageDir, "images.txt")

	// 检查每个镜像是否构建成功
	checkCmd := fmt.Sprintf("cat %s | while read image; do if ! docker images --format '{{.Repository}}:{{.Tag}}' | grep -q \"$image\"; then echo \"镜像构建失败: $image\"; exit 1; fi; done", imagesFile)

	cmd := exec.Command("bash", "-c", checkCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("镜像构建验证失败，部分镜像未成功构建")
	}

	addTaskLog(task, "所有镜像构建验证成功")
	return nil
}
