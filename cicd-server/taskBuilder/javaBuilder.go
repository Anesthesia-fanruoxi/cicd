package taskBuilder

import (
	"cicd-server/common"
	"cicd-server/config"
	packCode "cicd-server/javaTaskStep/4-packCode"
	getProduct "cicd-server/javaTaskStep/5-getProduct"
	createDockerFile "cicd-server/javaTaskStep/6-createDockerFile"
	buildImage "cicd-server/javaTaskStep/7-buildImage"
	pushImage "cicd-server/javaTaskStep/8-pushImage"
	"cicd-server/models"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// executeBuildImageTask 执行构建镜像任务
func ExecuteJavaBuildTask(task *models.Task, projectConfig *models.ProjectConfig) error {
	// 1. 初始化目录
	// 获取Git仓库地址
	gitURL := projectConfig.GitBackend // Java项目使用后端git仓库
	projectName := task.Name

	// 创建工作目录
	workspacePath := filepath.Join("/data/workspace", projectName)
	gitCloneDir := filepath.Join(workspacePath, "git_clone")
	logDir := filepath.Join(workspacePath, "logs")
	productDir := filepath.Join(workspacePath, "product")
	imageDir := filepath.Join(workspacePath, "image")
	timeDir := filepath.Join(workspacePath, "time")

	// 为每个任务创建独立的日志目录（在工作目录的logs下）
	taskLogDir := filepath.Join(logDir, task.ID)

	// 步骤1: 创建工作目录
	step1StartTime := time.Now()

	// 发送步骤1开始通知
	if err := common.NotifyTaskStepStart(task, 1, "init", "创建工作目录"); err != nil {
		LogToConsole(task, fmt.Sprintf("发送步骤1开始通知失败: %v", err))
	}

	dirs := []string{workspacePath, gitCloneDir, logDir, productDir, imageDir, timeDir, taskLogDir}
	step1Success := true
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			LogToConsole(task, fmt.Sprintf("创建目录失败: %s - %v", dir, err))
			step1Success = false
			break
		}
	}

	// 设置任务的日志目录开始处理子项目
	task.LogDir = taskLogDir

	// 发送步骤1完成通知（无论成功还是失败）
	if err := common.NotifyTaskStep(task, 1, "init", "创建工作目录", step1StartTime, time.Now(), step1Success); err != nil {
		LogToConsole(task, fmt.Sprintf("发送步骤1通知失败: %v", err))
	}

	// 如果步骤1失败，终止任务
	if !step1Success {
		return errors.New("创建工作目录失败")
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		return errors.New("任务被取消")
	default:
		// 继续执行
	}

	// 步骤2: 清理工作目录和旧镜像
	step2StartTime := time.Now()
	AddStepLog(task, "clear", "开始清理工作目录和旧镜像")

	// 发送步骤2开始通知
	if err := common.NotifyTaskStepStart(task, 2, "clear", "清理工作目录和旧镜像"); err != nil {
		AddStepLog(task, "clear", fmt.Sprintf("发送步骤2开始通知失败: %v", err))
	}

	// 清理工作目录
	if err := cleanWorkspaceDirectories(task, workspacePath); err != nil {
		AddStepLog(task, "clear", fmt.Sprintf("清理工作目录失败: %v", err))
		// 继续执行，不要中断
	}

	// 清除旧镜像
	harborDomain := config.GetHarborDomain()
	if err := cleanDockerImages(task, harborDomain, projectName); err != nil {
		AddStepLog(task, "clear", fmt.Sprintf("清除Docker镜像失败: %v", err))
		// 继续执行，不要中断
	}

	// 发送步骤2完成通知
	if err := common.NotifyTaskStep(task, 2, "clear", "清理工作目录和旧镜像", step2StartTime, time.Now(), true); err != nil {
		AddStepLog(task, "clear", fmt.Sprintf("发送步骤2通知失败: %v", err))
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		return errors.New("任务被取消")
	default:
		// 继续执行
	}

	// 步骤3: Git代码克隆
	step3StartTime := time.Now()
	AddStepLog(task, "git", "开始执行Git克隆")

	// 发送步骤3开始通知
	if err := common.NotifyTaskStepStart(task, 3, "git", "Git代码克隆"); err != nil {
		AddStepLog(task, "git", fmt.Sprintf("发送步骤3开始通知失败: %v", err))
	}

	// 清空Git克隆目录
	if err := os.RemoveAll(gitCloneDir); err != nil {
		AddStepLog(task, "git", fmt.Sprintf("清空Git克隆目录失败: %v", err))
	} else {
		AddStepLog(task, "git", "已清空Git克隆目录")
	}

	gitImage := "prohub.hzbxhd.com/library/git-clone:1.0" // 这里应该从配置中读取
	gitLogFile := filepath.Join(taskLogDir, "git.log")

	// 根据项目名称选择分支
	var gitCmd string
	if projectName == "bjjf-crm" {
		// bjjf-crm 项目使用 master_bajiejinfu 分支
		gitCmd = fmt.Sprintf("docker run --rm -v %s:/app -v /root/.ssh:/root/.ssh/ %s /bin/sh -c \"git clone --depth 1 -b master_bajiejinfu %s /app/git_clone && cd /app/git_clone && echo '=== 最后一次提交信息 ===' && git log -1 --pretty=format:'提交ID: %%H%%n作者: %%an <%%ae>%%n时间: %%ad%%n消息: %%s' --date=format:'%%Y-%%m-%%d %%H:%%M:%%S'\"",
			workspacePath, gitImage, gitURL)
		AddStepLog(task, "git", "使用分支: master_bajiejinfu")
	} else {
		// 其他项目使用默认分支
		gitCmd = fmt.Sprintf("docker run --rm -v %s:/app -v /root/.ssh:/root/.ssh/ %s /bin/sh -c \"git clone --depth 1 %s /app/git_clone && cd /app/git_clone && echo '=== 最后一次提交信息 ===' && git log -1 --pretty=format:'提交ID: %%H%%n作者: %%an <%%ae>%%n时间: %%ad%%n消息: %%s' --date=format:'%%Y-%%m-%%d %%H:%%M:%%S'\"",
			workspacePath, gitImage, gitURL)
	}

	// 添加调试信息
	LogToConsole(task, fmt.Sprintf("DEBUG: 即将执行命令: %s", gitCmd))
	AddStepLog(task, "git", fmt.Sprintf("执行命令: %s", gitCmd))
	AddStepLog(task, "git", fmt.Sprintf("Git日志输出到: %s", gitLogFile))

	step3Success := true
	step3EndTime := time.Now()

	// 执行Git克隆命令
	if err := ExecuteCommandWithLog(task, gitCmd, gitLogFile); err != nil {
		AddStepLog(task, "git", fmt.Sprintf("Git克隆失败: %v", err))
		step3Success = false
	} else {
		// 检查Git克隆是否成功
		if err := CheckGitCloneSuccess(gitCloneDir); err != nil {
			AddStepLog(task, "git", fmt.Sprintf("Git克隆验证失败: %v", err))
			step3Success = false
		} else {
			AddStepLog(task, "git", "Git克隆验证成功")
			step3EndTime = time.Now()
			gitDuration := step3EndTime.Sub(step3StartTime).Seconds()
			AddStepLog(task, "git", fmt.Sprintf("代码拉取完成，耗时: %.2f秒", gitDuration))

			// 记录Git拉取时间
			if err := AppendToFile(filepath.Join(timeDir, "ready_time.txt"),
				fmt.Sprintf("%.0f", gitDuration)); err != nil {
				AddStepLog(task, "git", fmt.Sprintf("写入Git拉取时间失败: %v", err))
			}
		}
	}

	// 发送步骤3完成通知（无论成功还是失败）
	if err := common.NotifyTaskStep(task, 3, "git", "Git代码克隆", step3StartTime, step3EndTime, step3Success); err != nil {
		AddStepLog(task, "git", fmt.Sprintf("发送步骤3通知失败: %v", err))
	}

	// 如果步骤3失败，终止任务
	if !step3Success {
		return errors.New("Git代码克隆失败")
	}

	// 步骤4: 编译代码
	step4StartTime := time.Now()
	AddStepLog(task, "mvn", "开始编译代码")

	// 发送步骤4开始通知
	if err := common.NotifyTaskStepStart(task, 4, "mvn", "编译代码"); err != nil {
		AddStepLog(task, "mvn", fmt.Sprintf("发送步骤4开始通知失败: %v", err))
	}

	// 获取项目的Java版本配置
	javaVersion := "java17" // 默认版本
	if projectConfig.BackendTool != "" {
		javaVersion = projectConfig.BackendTool
	}
	AddStepLog(task, "mvn", fmt.Sprintf("项目Java版本: %s", javaVersion))

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		AddStepLog(task, "mvn", "任务被取消")
		// 发送步骤4取消通知
		if err := common.NotifyTaskStepCancel(task, 4, "mvn", "编译代码", step4StartTime, time.Now()); err != nil {
			AddStepLog(task, "mvn", fmt.Sprintf("发送步骤4取消通知失败: %v", err))
		}
		return errors.New("任务被取消")
	default:
		// 继续执行
	}

	// 根据项目类型选择编译方式
	step4Success := true
	step4Canceled := false

	// 创建步骤日志函数
	step4LogFunc := CreateStepLogFunc(task, "mvn")

	if err := packCode.ExecuteJavaMavenPack(task, javaVersion, gitCloneDir, logDir, timeDir, taskLogDir, step4LogFunc, ExecuteCommand, ExecuteCommandWithLog, AppendToFile); err != nil {
		// 检查是否为取消错误
		if strings.Contains(err.Error(), "任务被取消") || strings.Contains(err.Error(), "命令被取消") {
			step4Canceled = true
		} else {
			step4Success = false
		}
	}

	// 发送步骤4结束通知（根据实际情况发送不同状态）
	if step4Canceled {
		// 发送取消通知
		if err := common.NotifyTaskStepCancel(task, 4, "mvn", "编译代码", step4StartTime, time.Now()); err != nil {
			AddStepLog(task, "mvn", fmt.Sprintf("发送步骤4取消通知失败: %v", err))
		}
	} else {
		// 发送成功或失败通知
		if err := common.NotifyTaskStep(task, 4, "mvn", "编译代码", step4StartTime, time.Now(), step4Success); err != nil {
			AddStepLog(task, "mvn", fmt.Sprintf("发送步骤4完成通知失败: %v", err))
		}
	}

	// 如果步骤被取消，返回取消错误
	if step4Canceled {
		return errors.New("任务被取消")
	}

	// 如果步骤4失败，终止任务
	if !step4Success {
		return errors.New("编译代码失败")
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		return fmt.Errorf("任务被取消")
	default:
		// 继续执行
	}

	// 步骤5: 获取产物
	step5StartTime := time.Now()
	AddStepLog(task, "product", "开始获取项目产物")

	// 发送步骤5开始通知
	if err := common.NotifyTaskStepStart(task, 5, "product", "获取项目产物"); err != nil {
		AddStepLog(task, "product", fmt.Sprintf("发送步骤5开始通知失败: %v", err))
	}

	step5Success := true
	step5Canceled := false

	// 创建步骤日志函数
	step5LogFunc := CreateStepLogFunc(task, "product")

	// 调用产物获取函数
	if err := getProduct.ExecuteProductAPI(task, projectName, gitCloneDir, productDir, taskLogDir, step5LogFunc, ExecuteCommand); err != nil {
		// 检查是否为取消错误
		if strings.Contains(err.Error(), "任务被取消") || strings.Contains(err.Error(), "命令被取消") {
			step5Canceled = true
		} else {
			step5Success = false
			AddStepLog(task, "product", fmt.Sprintf("获取项目产物失败: %v", err))
		}
	}

	// 发送步骤5结束通知（根据实际情况发送不同状态）
	if step5Canceled {
		// 发送取消通知
		if err := common.NotifyTaskStepCancel(task, 5, "product", "获取项目产物", step5StartTime, time.Now()); err != nil {
			AddStepLog(task, "product", fmt.Sprintf("发送步骤5取消通知失败: %v", err))
		}
		return fmt.Errorf("任务被取消")
	} else {
		// 发送成功或失败通知
		if err := common.NotifyTaskStep(task, 5, "product", "获取项目产物", step5StartTime, time.Now(), step5Success); err != nil {
			AddStepLog(task, "product", fmt.Sprintf("发送步骤5完成通知失败: %v", err))
		}
	}

	// 如果步骤5失败，终止任务
	if !step5Success {
		return fmt.Errorf("获取项目产物失败")
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		return fmt.Errorf("任务被取消")
	default:
		// 继续执行
	}

	// 步骤6: 创建Dockerfile
	step6StartTime := time.Now()
	AddStepLog(task, "dockerfile", "开始创建Dockerfile")

	// 发送步骤6开始通知
	if err := common.NotifyTaskStepStart(task, 6, "dockerfile", "创建Dockerfile"); err != nil {
		AddStepLog(task, "dockerfile", fmt.Sprintf("发送步骤6开始通知失败: %v", err))
	}

	step6Success := true
	step6Canceled := false

	// 生成时间戳作为镜像标签
	timestamp := time.Now().Format("20060102150405")

	// 设置任务的镜像tag
	task.ImageTag = timestamp

	// 创建步骤日志函数
	step6LogFunc := CreateStepLogFunc(task, "dockerfile")

	// 调用Dockerfile创建函数
	if err := createDockerFile.ExecuteJavaDockerfile(task, projectName, productDir, imageDir, taskLogDir, javaVersion, timestamp, step6LogFunc, ExecuteCommand); err != nil {
		// 检查是否为取消错误
		if strings.Contains(err.Error(), "任务被取消") || strings.Contains(err.Error(), "命令被取消") {
			step6Canceled = true
		} else {
			step6Success = false
			AddStepLog(task, "dockerfile", fmt.Sprintf("创建Dockerfile失败: %v", err))
		}
	}

	// 发送步骤6结束通知（根据实际情况发送不同状态）
	if step6Canceled {
		// 发送取消通知
		if err := common.NotifyTaskStepCancel(task, 6, "dockerfile", "创建Dockerfile", step6StartTime, time.Now()); err != nil {
			AddStepLog(task, "dockerfile", fmt.Sprintf("发送步骤6取消通知失败: %v", err))
		}
		return fmt.Errorf("任务被取消")
	} else {
		// 发送成功或失败通知
		if err := common.NotifyTaskStep(task, 6, "dockerfile", "创建Dockerfile", step6StartTime, time.Now(), step6Success); err != nil {
			AddStepLog(task, "dockerfile", fmt.Sprintf("发送步骤6完成通知失败: %v", err))
		}
	}

	// 如果步骤6失败，终止任务
	if !step6Success {
		return fmt.Errorf("创建Dockerfile失败")
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		return fmt.Errorf("任务被取消")
	default:
		// 继续执行
	}

	// 步骤7: 构建Docker镜像
	step7StartTime := time.Now()
	AddStepLog(task, "build", "开始构建Docker镜像")

	// 发送步骤7开始通知
	if err := common.NotifyTaskStepStart(task, 7, "build", "构建Docker镜像"); err != nil {
		AddStepLog(task, "build", fmt.Sprintf("发送步骤7开始通知失败: %v", err))
	}

	step7Success := true
	step7Canceled := false

	// 创建步骤日志函数
	step7LogFunc := CreateStepLogFunc(task, "build")

	// 调用镜像构建函数
	if err := buildImage.ExecuteBuildImage(task, projectName, imageDir, taskLogDir, step7LogFunc, ExecuteCommand, ExecuteCommandWithLog); err != nil {
		// 检查是否为取消错误
		if strings.Contains(err.Error(), "任务被取消") || strings.Contains(err.Error(), "命令被取消") {
			step7Canceled = true
		} else {
			step7Success = false
			AddStepLog(task, "build", fmt.Sprintf("构建Docker镜像失败: %v", err))
		}
	}

	// 发送步骤7结束通知（根据实际情况发送不同状态）
	if step7Canceled {
		// 发送取消通知
		if err := common.NotifyTaskStepCancel(task, 7, "build", "构建Docker镜像", step7StartTime, time.Now()); err != nil {
			AddStepLog(task, "build", fmt.Sprintf("发送步骤7取消通知失败: %v", err))
		}
		return fmt.Errorf("任务被取消")
	} else {
		// 发送成功或失败通知
		if err := common.NotifyTaskStep(task, 7, "build", "构建Docker镜像", step7StartTime, time.Now(), step7Success); err != nil {
			AddStepLog(task, "build", fmt.Sprintf("发送步骤7完成通知失败: %v", err))
		}
	}

	// 如果步骤7失败，终止任务
	if !step7Success {
		return fmt.Errorf("构建Docker镜像失败")
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		return fmt.Errorf("任务被取消")
	default:
		// 继续执行
	}

	// 步骤8: 推送Docker镜像
	step8StartTime := time.Now()
	AddStepLog(task, "push", "开始推送Docker镜像")

	// 发送步骤8开始通知
	if err := common.NotifyTaskStepStart(task, 8, "push", "推送Docker镜像"); err != nil {
		AddStepLog(task, "push", fmt.Sprintf("发送步骤8开始通知失败: %v", err))
	}

	step8Success := true
	step8Canceled := false

	// 创建步骤日志函数
	step8LogFunc := CreateStepLogFunc(task, "push")

	// 调用镜像推送函数
	if err := pushImage.ExecutePushImage(task, projectName, imageDir, taskLogDir, step8LogFunc, ExecuteCommand, ExecuteCommandWithLog); err != nil {
		// 检查是否为取消错误
		if strings.Contains(err.Error(), "任务被取消") || strings.Contains(err.Error(), "命令被取消") {
			step8Canceled = true
		} else {
			step8Success = false
			AddStepLog(task, "push", fmt.Sprintf("推送Docker镜像失败: %v", err))
		}
	}

	// 发送步骤8结束通知（根据实际情况发送不同状态）
	if step8Canceled {
		// 发送取消通知
		if err := common.NotifyTaskStepCancel(task, 8, "push", "推送Docker镜像", step8StartTime, time.Now()); err != nil {
			AddStepLog(task, "push", fmt.Sprintf("发送步骤8取消通知失败: %v", err))
		}
		return fmt.Errorf("任务被取消")
	} else {
		// 发送成功或失败通知
		if err := common.NotifyTaskStep(task, 8, "push", "推送Docker镜像", step8StartTime, time.Now(), step8Success); err != nil {
			AddStepLog(task, "push", fmt.Sprintf("发送步骤8完成通知失败: %v", err))
		}
	}

	// 如果步骤8失败，终止任务
	if !step8Success {
		return fmt.Errorf("推送Docker镜像失败")
	}

	return nil
}

// cleanWorkspaceDirectories 清理工作目录
func cleanWorkspaceDirectories(task *models.Task, workspacePath string) error {
	AddStepLog(task, "clear", "开始清理工作目录")

	// 定义需要清理的目录列表（后端项目）
	dirsToClean := []string{"git_clone", "image", "product"}

	// 检查是否为前端项目（没有image目录）
	imagePath := filepath.Join(workspacePath, "image")
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		// 前端项目目录结构
		dirsToClean = []string{"git_clone", "product"}
		AddStepLog(task, "clear", "检测到前端项目，使用前端目录清理规则")
	} else {
		AddStepLog(task, "clear", "检测到后端项目，使用后端目录清理规则")
	}

	// 清理指定目录（清空内容但保留目录）
	for _, dirName := range dirsToClean {
		dirPath := filepath.Join(workspacePath, dirName)
		if _, err := os.Stat(dirPath); err == nil {
			AddStepLog(task, "clear", fmt.Sprintf("清空目录内容: %s", dirPath))
			if err := clearDirectoryContents(task, dirPath); err != nil {
				AddStepLog(task, "clear", fmt.Sprintf("清空目录内容失败 %s: %v", dirPath, err))
			} else {
				AddStepLog(task, "clear", fmt.Sprintf("成功清空目录内容: %s", dirPath))
			}
		} else {
			AddStepLog(task, "clear", fmt.Sprintf("目录不存在，跳过: %s", dirPath))
		}
	}

	// 特殊处理logs目录 - 只删除一个月前的日志
	logsPath := filepath.Join(workspacePath, "logs")
	if err := cleanOldLogs(task, logsPath); err != nil {
		AddStepLog(task, "clear", fmt.Sprintf("清理旧日志失败: %v", err))
	}

	AddStepLog(task, "clear", "工作目录清理完成")
	return nil
}

// cleanOldLogs 清理一个月前的日志文件
func cleanOldLogs(task *models.Task, logsPath string) error {
	if _, err := os.Stat(logsPath); os.IsNotExist(err) {
		AddStepLog(task, "clear", "logs目录不存在，跳过日志清理")
		return nil
	}

	AddStepLog(task, "clear", "开始清理一个月前的日志文件")

	// 计算一个月前的时间
	oneMonthAgo := time.Now().AddDate(0, -1, 0)

	// 遍历logs目录
	entries, err := os.ReadDir(logsPath)
	if err != nil {
		return fmt.Errorf("读取logs目录失败: %v", err)
	}

	deletedCount := 0
	for _, entry := range entries {
		entryPath := filepath.Join(logsPath, entry.Name())

		// 获取文件/目录信息
		info, err := entry.Info()
		if err != nil {
			AddStepLog(task, "clear", fmt.Sprintf("获取文件信息失败 %s: %v", entryPath, err))
			continue
		}

		// 检查修改时间是否超过一个月
		if info.ModTime().Before(oneMonthAgo) {
			AddStepLog(task, "clear", fmt.Sprintf("删除过期日志: %s (修改时间: %s)", entryPath, info.ModTime().Format("2006-01-02 15:04:05")))

			if err := os.RemoveAll(entryPath); err != nil {
				AddStepLog(task, "clear", fmt.Sprintf("删除过期日志失败 %s: %v", entryPath, err))
			} else {
				deletedCount++
			}
		}
	}

	AddStepLog(task, "clear", fmt.Sprintf("日志清理完成，共删除 %d 个过期文件/目录", deletedCount))
	return nil
}

// clearDirectoryContents 清空目录内容但保留目录本身
func clearDirectoryContents(task *models.Task, dirPath string) error {
	// 读取目录内容
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("读取目录失败: %v", err)
	}

	// 删除目录内的所有文件和子目录
	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			AddStepLog(task, "clear", fmt.Sprintf("删除文件/目录失败 %s: %v", entryPath, err))
		}
	}

	return nil
}

// cleanDockerImages 清理项目的Docker历史镜像
func cleanDockerImages(task *models.Task, harborDomain, projectName string) error {
	AddStepLog(task, "clear", "开始清理Docker历史镜像")

	// 构建镜像搜索模式
	searchPattern := fmt.Sprintf("%s/%s", harborDomain, projectName)
	AddStepLog(task, "clear", fmt.Sprintf("查找镜像: %s", searchPattern))

	// 先列出要删除的镜像
	listCmd := fmt.Sprintf("docker images --format '{{.Repository}}:{{.Tag}}' | grep %s", searchPattern)
	cmd := exec.Command("bash", "-c", listCmd)
	output, err := cmd.Output()

	if err != nil || len(output) == 0 {
		AddStepLog(task, "clear", "未找到需要清理的历史镜像")
		return nil
	}

	// 输出找到的镜像
	images := strings.Split(strings.TrimSpace(string(output)), "\n")
	AddStepLog(task, "clear", fmt.Sprintf("找到 %d 个历史镜像需要清理:", len(images)))
	for _, img := range images {
		if img != "" {
			AddStepLog(task, "clear", fmt.Sprintf("  - %s", img))
		}
	}

	// 执行删除
	deleteCmd := fmt.Sprintf("docker images --format '{{.Repository}}:{{.Tag}}' | grep %s | xargs -r docker rmi",
		searchPattern)
	deleteExec := exec.Command("bash", "-c", deleteCmd)
	deleteOutput, deleteErr := deleteExec.CombinedOutput()

	if deleteErr != nil {
		AddStepLog(task, "clear", fmt.Sprintf("清理镜像时出现警告: %v", deleteErr))
		if len(deleteOutput) > 0 {
			AddStepLog(task, "clear", fmt.Sprintf("输出: %s", string(deleteOutput)))
		}
		return nil // 不中断流程
	}

	AddStepLog(task, "clear", fmt.Sprintf("成功清理 %d 个Docker历史镜像", len(images)))
	return nil
}
