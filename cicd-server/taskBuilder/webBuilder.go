package taskBuilder

import (
	"cicd-server/common"
	"cicd-server/models"
	installDependencies "cicd-server/webTaskStep/4-installDependencies"
	webPack "cicd-server/webTaskStep/5-webPack"
	getProduct "cicd-server/webTaskStep/6-getProduct"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// executeWebBuildTask 执行Web项目构建任务
func ExecuteWebBuildTask(task *models.Task, projectConfig *models.ProjectConfig) error {
	// 1. 初始化目录
	gitURL := projectConfig.GitVue // Web项目使用前端git仓库
	projectName := task.Name

	// 创建工作目录（前端项目添加-web后缀以区分后端项目）
	workspacePath := filepath.Join("/data/workspace", projectName+"-web")
	gitCloneDir := filepath.Join(workspacePath, "git_clone")
	logDir := filepath.Join(workspacePath, "logs")
	productDir := filepath.Join(workspacePath, "product")
	timeDir := filepath.Join(workspacePath, "time")

	// 为每个任务创建独立的日志目录
	taskLogDir := filepath.Join(logDir, task.ID)

	// 生成时间戳作为构建标签
	timestamp := time.Now().Format("20060102150405")
	task.ImageTag = timestamp

	// 步骤1: 创建工作目录
	step1StartTime := time.Now()

	if err := common.NotifyTaskStepStart(task, 1, "init", "创建工作目录"); err != nil {
		LogToConsole(task, fmt.Sprintf("发送步骤1开始通知失败: %v", err))
	}

	dirs := []string{workspacePath, gitCloneDir, logDir, productDir, timeDir, taskLogDir}
	step1Success := true
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			LogToConsole(task, fmt.Sprintf("创建目录失败: %s - %v", dir, err))
			step1Success = false
			break
		}
	}

	// 设置任务的日志目录
	task.LogDir = taskLogDir

	if err := common.NotifyTaskStep(task, 1, "init", "创建工作目录", step1StartTime, time.Now(), step1Success); err != nil {
		LogToConsole(task, fmt.Sprintf("发送步骤1通知失败: %v", err))
	}

	if !step1Success {
		return errors.New("创建工作目录失败")
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		return errors.New("任务被取消")
	default:
	}

	// 步骤2: 清理工作目录
	step2StartTime := time.Now()
	AddStepLog(task, "clear", "开始清理工作目录")

	if err := common.NotifyTaskStepStart(task, 2, "clear", "清理工作目录"); err != nil {
		AddStepLog(task, "clear", fmt.Sprintf("发送步骤2开始通知失败: %v", err))
	}

	// 清理工作目录
	if err := cleanWorkspaceDirectories(task, workspacePath); err != nil {
		AddStepLog(task, "clear", fmt.Sprintf("清理工作目录失败: %v", err))
		// 继续执行，不要中断
	}

	if err := common.NotifyTaskStep(task, 2, "clear", "清理工作目录", step2StartTime, time.Now(), true); err != nil {
		AddStepLog(task, "clear", fmt.Sprintf("发送步骤2通知失败: %v", err))
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		return errors.New("任务被取消")
	default:
	}

	// 步骤3: Git代码克隆
	step3StartTime := time.Now()
	AddStepLog(task, "git", "开始执行Git克隆")

	if err := common.NotifyTaskStepStart(task, 3, "git", "Git代码克隆"); err != nil {
		AddStepLog(task, "git", fmt.Sprintf("发送步骤3开始通知失败: %v", err))
	}

	gitImage := "prohub.hzbxhd.com/library/git-clone:1.0"
	gitLogFile := filepath.Join(taskLogDir, "git.log")
	gitCmd := fmt.Sprintf("docker run --rm -v %s:/app -v /root/.ssh:/root/.ssh/ %s /bin/sh -c \"git clone --depth 1 %s /app/git_clone && cd /app/git_clone && echo '=== 最后一次提交信息 ===' && git log -1 --pretty=format:'提交ID: %%H%%n作者: %%an <%%ae>%%n时间: %%ad%%n消息: %%s' --date=format:'%%Y-%%m-%%d %%H:%%M:%%S'\"",
		workspacePath, gitImage, gitURL)

	AddStepLog(task, "git", fmt.Sprintf("执行命令: %s", gitCmd))
	AddStepLog(task, "git", fmt.Sprintf("Git日志输出到: %s", gitLogFile))

	step3Success := true
	step3EndTime := time.Now()

	if err := ExecuteCommandWithLog(task, gitCmd, gitLogFile); err != nil {
		AddStepLog(task, "git", fmt.Sprintf("Git克隆失败: %v", err))
		step3Success = false
	} else {
		if err := CheckGitCloneSuccess(gitCloneDir); err != nil {
			AddStepLog(task, "git", fmt.Sprintf("Git克隆验证失败: %v", err))
			step3Success = false
		} else {
			AddStepLog(task, "git", "Git克隆验证成功")
			step3EndTime = time.Now()
			gitDuration := step3EndTime.Sub(step3StartTime).Seconds()
			AddStepLog(task, "git", fmt.Sprintf("代码拉取完成，耗时: %.2f秒", gitDuration))

			if err := AppendToFile(filepath.Join(timeDir, "ready_time.txt"),
				fmt.Sprintf("%.0f", gitDuration)); err != nil {
				AddStepLog(task, "git", fmt.Sprintf("写入Git拉取时间失败: %v", err))
			}
		}
	}

	if err := common.NotifyTaskStep(task, 3, "git", "Git代码克隆", step3StartTime, step3EndTime, step3Success); err != nil {
		AddStepLog(task, "git", fmt.Sprintf("发送步骤3通知失败: %v", err))
	}

	if !step3Success {
		return errors.New("Git代码克隆失败")
	}

	// 步骤4: 安装依赖
	step4StartTime := time.Now()
	AddStepLog(task, "npm", "开始安装Node.js依赖")

	if err := common.NotifyTaskStepStart(task, 4, "npm", "安装Node.js依赖"); err != nil {
		AddStepLog(task, "npm", fmt.Sprintf("发送步骤4开始通知失败: %v", err))
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		if err := common.NotifyTaskStepCancel(task, 4, "npm", "安装Node.js依赖", step4StartTime, time.Now()); err != nil {
			AddStepLog(task, "npm", fmt.Sprintf("发送步骤4取消通知失败: %v", err))
		}
		return errors.New("任务被取消")
	default:
	}

	step4Success := true
	step4Canceled := false

	// 创建步骤4日志函数
	step4LogFunc := CreateStepLogFunc(task, "npm")

	// 执行依赖安装
	if err := installDependencies.ExecuteInstallDependencies(task, "node18", gitCloneDir, logDir, timeDir, taskLogDir, step4LogFunc, ExecuteCommand, ExecuteCommandWithLog, AppendToFile); err != nil {
		if strings.Contains(err.Error(), "任务被取消") || strings.Contains(err.Error(), "命令被取消") {
			step4Canceled = true
		} else {
			step4Success = false
		}
	}

	if step4Canceled {
		if err := common.NotifyTaskStepCancel(task, 4, "npm", "安装Node.js依赖", step4StartTime, time.Now()); err != nil {
			AddStepLog(task, "npm", fmt.Sprintf("发送步骤4取消通知失败: %v", err))
		}
		return errors.New("任务被取消")
	} else {
		if err := common.NotifyTaskStep(task, 4, "npm", "安装Node.js依赖", step4StartTime, time.Now(), step4Success); err != nil {
			AddStepLog(task, "npm", fmt.Sprintf("发送步骤4完成通知失败: %v", err))
		}
	}

	if !step4Success {
		return errors.New("安装Node.js依赖失败")
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		return fmt.Errorf("任务被取消")
	default:
	}

	// 步骤5: Web项目打包
	step5StartTime := time.Now()
	AddStepLog(task, "build", "开始Web项目打包")

	if err := common.NotifyTaskStepStart(task, 5, "build", "Web项目打包"); err != nil {
		AddStepLog(task, "build", fmt.Sprintf("发送步骤5开始通知失败: %v", err))
	}

	step5Success := true
	step5Canceled := false

	// 创建步骤5日志函数
	step5LogFunc := CreateStepLogFunc(task, "build")

	// 执行Web项目打包
	if err := webPack.ExecuteWebPack(task, "node18", gitCloneDir, logDir, timeDir, taskLogDir, step5LogFunc, ExecuteCommand, ExecuteCommandWithLog, AppendToFile); err != nil {
		if strings.Contains(err.Error(), "任务被取消") || strings.Contains(err.Error(), "命令被取消") {
			step5Canceled = true
		} else {
			step5Success = false
		}
	}

	if step5Canceled {
		if err := common.NotifyTaskStepCancel(task, 5, "build", "Web项目打包", step5StartTime, time.Now()); err != nil {
			AddStepLog(task, "build", fmt.Sprintf("发送步骤5取消通知失败: %v", err))
		}
		return errors.New("任务被取消")
	} else {
		if err := common.NotifyTaskStep(task, 5, "build", "Web项目打包", step5StartTime, time.Now(), step5Success); err != nil {
			AddStepLog(task, "build", fmt.Sprintf("发送步骤5完成通知失败: %v", err))
		}
	}

	if !step5Success {
		return errors.New("Web项目打包失败")
	}

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		return fmt.Errorf("任务被取消")
	default:
	}

	// 步骤6: 获取产物并上传
	step6StartTime := time.Now()
	AddStepLog(task, "product", "开始获取Web项目产物")

	if err := common.NotifyTaskStepStart(task, 6, "product", "获取Web项目产物"); err != nil {
		AddStepLog(task, "product", fmt.Sprintf("发送步骤6开始通知失败: %v", err))
	}

	step6Success := true
	step6Canceled := false

	// 创建步骤6日志函数
	step6LogFunc := CreateStepLogFunc(task, "product")

	if err := getProduct.ExecuteWebProduct(task, projectName, gitCloneDir, productDir, taskLogDir, step6LogFunc, ExecuteCommand); err != nil {
		if strings.Contains(err.Error(), "任务被取消") || strings.Contains(err.Error(), "命令被取消") {
			step6Canceled = true
		} else {
			step6Success = false
			AddStepLog(task, "product", fmt.Sprintf("获取Web项目产物失败: %v", err))
		}
	}

	if step6Canceled {
		if err := common.NotifyTaskStepCancel(task, 6, "product", "获取Web项目产物", step6StartTime, time.Now()); err != nil {
			AddStepLog(task, "product", fmt.Sprintf("发送步骤6取消通知失败: %v", err))
		}
		return fmt.Errorf("任务被取消")
	} else {
		if err := common.NotifyTaskStep(task, 6, "product", "获取Web项目产物", step6StartTime, time.Now(), step6Success); err != nil {
			AddStepLog(task, "product", fmt.Sprintf("发送步骤6完成通知失败: %v", err))
		}
	}

	if !step6Success {
		return fmt.Errorf("获取Web项目产物失败")
	}

	return nil
}
