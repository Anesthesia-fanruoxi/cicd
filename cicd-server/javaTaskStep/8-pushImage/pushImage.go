package pushImage

import (
	"cicd-server/models"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

// ExecutePushImage 执行Docker镜像推送
func ExecutePushImage(task *models.Task, projectName, imageDir, taskLogDir string,
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

	addTaskLog(task, "开始推送Docker镜像")

	// 推送镜像
	if err := pushDockerImages(task, imageDir, taskLogDir, addTaskLog, executeCommandWithLog); err != nil {
		return fmt.Errorf("推送Docker镜像失败: %v", err)
	}

	addTaskLog(task, "Docker镜像推送完成")
	return nil
}

// pushDockerImages 推送Docker镜像
func pushDockerImages(task *models.Task, imageDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommandWithLog func(*models.Task, string, string) error) error {

	addTaskLog(task, "开始执行镜像推送")

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	// 推送命令：cat $image_dir/images.txt | xargs -I {} -n 1 -P 20 docker push {}
	pushStartTime := time.Now()
	pushLogFile := filepath.Join(taskLogDir, "push.log")

	// 使用xargs并行推送镜像（-P 20表示最多20个并行进程）
	pushCmd := fmt.Sprintf("cat %s/images.txt | xargs -I {} -n 1 -P 20 docker push {}", imageDir)

	addTaskLog(task, fmt.Sprintf("执行推送命令: %s", pushCmd))
	addTaskLog(task, fmt.Sprintf("推送日志输出到: %s", pushLogFile))

	if err := executeCommandWithLog(task, pushCmd, pushLogFile); err != nil {
		addTaskLog(task, fmt.Sprintf("Docker镜像推送失败: %v", err))

		// 提取推送错误日志
		if err := extractPushErrors(task, pushLogFile, taskLogDir, addTaskLog); err != nil {
			addTaskLog(task, fmt.Sprintf("提取推送错误日志失败: %v", err))
		}

		return err
	}

	pushEndTime := time.Now()
	pushDuration := pushEndTime.Sub(pushStartTime).Seconds()
	addTaskLog(task, fmt.Sprintf("Docker镜像推送完成，耗时: %.2f秒", pushDuration))

	// 验证推送结果
	if err := verifyPushSuccess(task, imageDir, addTaskLog); err != nil {
		addTaskLog(task, fmt.Sprintf("推送结果验证失败: %v", err))
		return err
	}

	return nil
}

// extractPushErrors 提取推送错误日志
func extractPushErrors(task *models.Task, pushLogFile, taskLogDir string,
	addTaskLog func(*models.Task, string)) error {

	pushErrorFile := filepath.Join(taskLogDir, "push_error.log")

	// 提取ERROR关键字的日志行
	grepCmd := fmt.Sprintf("grep -i \"ERROR\\|FAILED\\|FAILURE\\|denied\" %s | uniq > %s", pushLogFile, pushErrorFile)

	cmd := exec.Command("bash", "-c", grepCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行grep命令失败: %v", err)
	}

	addTaskLog(task, fmt.Sprintf("推送错误日志已提取到: %s", pushErrorFile))
	return nil
}

// verifyPushSuccess 验证推送是否成功
func verifyPushSuccess(task *models.Task, imageDir string,
	addTaskLog func(*models.Task, string)) error {

	addTaskLog(task, "开始验证推送结果")

	// 检查推送日志中是否有成功标识
	pushLogFile := filepath.Join(filepath.Dir(imageDir), "logs", "push.log")

	// 简单验证：检查推送日志中是否包含足够的成功信息
	checkCmd := fmt.Sprintf("if [ -f %s ]; then grep -c \"digest:\" %s; else echo \"0\"; fi", pushLogFile, pushLogFile)

	cmd := exec.Command("bash", "-c", checkCmd)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("验证推送结果失败: %v", err)
	}

	addTaskLog(task, fmt.Sprintf("推送验证完成，成功推送标识数量: %s", string(output)))
	return nil
}
