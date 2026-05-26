package taskBuilder

import (
	"cicd-server/common"
	"cicd-server/config"
	"cicd-server/models"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	manager     *models.TaskManager
	managerOnce sync.Once
)

// GetTaskManager 获取任务管理器实例（单例模式）
func GetTaskManager() *models.TaskManager {
	managerOnce.Do(func() {
		// 使用固定种子初始化随机数生成器
		source := rand.NewSource(time.Now().UnixNano())
		rnd := rand.New(source)

		manager = &models.TaskManager{
			Tasks:       make(map[string]*models.Task),
			WorkerPools: make(map[models.TaskType]*models.WorkerPool),
			Random:      rnd,
		}

		// 初始化三种类型的工作线程池
		manager.WorkerPools[models.TaskTypeDefault] = &models.WorkerPool{
			Queue:       []*models.Task{},
			MaxWorkers:  1,
			WorkerCount: 0,
		}
		manager.WorkerPools[models.TaskTypeWeb] = &models.WorkerPool{
			Queue:       []*models.Task{},
			MaxWorkers:  1,
			WorkerCount: 0,
		}
		manager.WorkerPools[models.TaskTypeCrm] = &models.WorkerPool{
			Queue:       []*models.Task{},
			MaxWorkers:  1,
			WorkerCount: 0,
		}
	})
	return manager
}

// 生成唯一任务ID
func generateTaskID(tm *models.TaskManager) string {
	// 使用当前时间和随机字符串生成任务ID
	return time.Now().Format("20060102150405") + "-" + randomString(tm, 6)
}

// 生成随机字符串
func randomString(tm *models.TaskManager, n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[tm.Random.Intn(len(letters))]
	}
	return string(b)
}

// CreateTask 创建任务
// taskType: 前端传递的任务类型（web/double/single）
func CreateTask(name string, taskType string, callbackURL string, category string, description string, createdBy int, createdByName string) (*models.Task, error) {
	tm := GetTaskManager()

	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	// 根据taskType和项目名称确定工作池类型
	poolType := models.TaskTypeDefault
	if taskType == "web" {
		poolType = models.TaskTypeWeb
	} else if models.IsCrmProject(name) {
		// CRM项目使用CRM池
		poolType = models.TaskTypeCrm
	}

	// 获取项目信息
	var projectInfo *models.ProjectConfig
	projectConfig, err := config.GetProjectConfig(name)
	if err != nil {
	} else {
		projectInfo = projectConfig
		common.Logger.Infof("获取项目 %s 配置成功: %s", name, projectInfo.Description)
	}

	task := &models.Task{
		ID:            generateTaskID(tm),
		Name:          name,
		Type:          taskType,
		Status:        models.TaskStatusPending,
		CreatedAt:     time.Now(),
		CancelChan:    make(chan struct{}),
		ProjectInfo:   projectInfo,
		CallbackURL:   callbackURL,
		StepDurations: make(map[string]float64),
		Category:      category,
		Description:   description,
		CreatedBy:     createdBy,
		CreatedByName: createdByName,
	}

	// 将任务加入队列和任务列表
	tm.Tasks[task.ID] = task
	tm.WorkerPools[poolType].Queue = append(tm.WorkerPools[poolType].Queue, task)
	common.Logger.Infof("任务 %s [%s] 类型 [%s] 创建成功", task.Name, task.ID, task.Type)

	// 立即发送任务创建通知
	if err := common.NotifyTaskCreate(task); err != nil {
		common.Logger.Errorf("发送任务创建通知失败: %v", err)
	}

	// 发送飞书通知
	if err := common.SendFeishuTaskCreateNotification(task); err != nil {
		common.Logger.Errorf("发送飞书任务创建通知失败: %v", err)
	}

	// 尝试处理任务队列
	go processQueue(poolType)

	return task, nil
}

// GetTaskList 查询任务列表
func GetTaskList() []*models.Task {
	tm := GetTaskManager()

	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	taskList := make([]*models.Task, 0, len(tm.Tasks))
	for _, task := range tm.Tasks {
		taskList = append(taskList, task)
	}
	return taskList
}

// GetTaskListByType 按类型查询任务列表
func GetTaskListByType(taskType string) ([]*models.Task, error) {
	tm := GetTaskManager()

	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	taskList := make([]*models.Task, 0)
	for _, task := range tm.Tasks {
		if task.Type == taskType {
			taskList = append(taskList, task)
		}
	}
	return taskList, nil
}

// GetTaskByID 查询任务详情
func GetTaskByID(id string) (*models.Task, error) {
	tm := GetTaskManager()

	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	task, exists := tm.Tasks[id]
	if !exists {
		return nil, errors.New("任务不存在")
	}
	return task, nil
}

// CancelTask 取消任务
func CancelTask(id string) error {
	tm := GetTaskManager()

	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	task, exists := tm.Tasks[id]
	if !exists {
		return errors.New("任务不存在")
	}

	// 根据任务类型和项目名称确定工作池
	poolType := models.TaskTypeDefault
	if task.Type == "web" {
		poolType = models.TaskTypeWeb
	} else if models.IsCrmProject(task.Name) {
		// CRM项目使用CRM池
		poolType = models.TaskTypeCrm
	}

	// 根据任务状态执行不同的取消逻辑
	switch task.Status {
	case models.TaskStatusPending:
		// 如果任务还在等待，直接从队列中移除
		pool := tm.WorkerPools[poolType]
		for i, t := range pool.Queue {
			if t.ID == id {
				pool.Queue = append(pool.Queue[:i], pool.Queue[i+1:]...)
				break
			}
		}
		task.Status = models.TaskStatusCanceled
		now := time.Now()
		task.FinishedAt = &now
		common.Logger.Infof("任务 %s [%s] 已取消", task.Name, task.ID)

		// 发送任务完成通知（状态为canceled）
		if err := common.NotifyTaskComplete(task); err != nil {
			common.Logger.Errorf("发送任务完成通知失败: %v", err)
		}

		// 发送飞书完成通知
		if err := common.SendFeishuTaskCompleteNotification(task); err != nil {
			common.Logger.Errorf("发送飞书任务完成通知失败: %v", err)
		}
		return nil
	case models.TaskStatusRunning:
		// 如果任务正在运行，发送取消信号
		if task.CancelFunc != nil {
			task.CancelFunc()
		}
		close(task.CancelChan)
		task.Status = models.TaskStatusCanceled
		now := time.Now()
		task.FinishedAt = &now
		common.Logger.Infof("任务 %s [%s] 已取消", task.Name, task.ID)

		// 发送任务完成通知（状态为canceled）
		if err := common.NotifyTaskComplete(task); err != nil {
			common.Logger.Errorf("发送任务完成通知失败: %v", err)
		}

		// 发送飞书完成通知
		if err := common.SendFeishuTaskCompleteNotification(task); err != nil {
			common.Logger.Errorf("发送飞书任务完成通知失败: %v", err)
		}
		return nil
	default:
		// 已完成、失败或已取消的任务不能再取消
		return errors.New("无法取消该状态的任务")
	}
}

// processQueue 处理任务队列
func processQueue(taskType models.TaskType) {
	tm := GetTaskManager()

	tm.Mutex.Lock()
	defer tm.Mutex.Unlock()

	pool, exists := tm.WorkerPools[taskType]
	if !exists {
		common.Logger.Errorf("无效的任务类型: %s", taskType)
		return
	}

	// 如果没有等待的任务或工作线程已达到最大数量，直接返回
	if len(pool.Queue) == 0 || pool.WorkerCount >= pool.MaxWorkers {
		return
	}

	// 取出队列第一个任务
	task := pool.Queue[0]
	pool.Queue = pool.Queue[1:]
	pool.WorkerCount++

	// 更新任务状态为运行中
	task.Status = models.TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	common.Logger.Infof("任务 %s [%s] 类型 [%s] 开始执行", task.Name, task.ID, task.Type)

	// 创建取消函数
	done := make(chan struct{})
	var doneOnce sync.Once
	task.CancelFunc = func() {
		doneOnce.Do(func() {
			close(done)
		})
	}

	// 启动goroutine执行任务
	go func() {
		defer func() {
			// 任务完成后，减少工作线程数量并处理队列
			tm.Mutex.Lock()
			pool.WorkerCount--
			tm.Mutex.Unlock()

			// 安全关闭 done channel
			doneOnce.Do(func() {
				close(done)
			})

			// 尝试处理下一个任务
			go processQueue(taskType)
		}()

		// 获取项目配置
		var err error
		projectConfig, configErr := config.GetProjectConfig(task.Name)
		if configErr != nil {
			LogToConsole(task, fmt.Sprintf("获取项目配置失败: %v", configErr))
			err = configErr
		} else {
			// 执行具体的任务逻辑
			err = ExecuteTask(task, projectConfig)
		}

		tm.Mutex.Lock()
		defer tm.Mutex.Unlock()

		// 如果任务被取消，不更新状态也不发送通知（通知已在CancelTask中发送）
		if task.Status == models.TaskStatusCanceled {
			return
		}

		// 更新任务状态
		now := time.Now()
		task.FinishedAt = &now

		if err != nil {
			// 检查是否为取消错误
			if strings.Contains(err.Error(), "任务被取消") || strings.Contains(err.Error(), "命令被取消") {
				// 如果是取消错误但状态还不是canceled，说明是执行过程中被取消的
				// 这种情况下不应该发生，因为CancelTask应该已经设置了状态
				common.Logger.Warnf("任务 %s [%s] 执行过程中被取消，但状态未及时更新", task.Name, task.ID)
				return // 不发送重复通知
			} else {
				task.Status = models.TaskStatusFailed
				task.Result = "执行失败: " + err.Error()
				common.Logger.Errorf("任务 %s [%s] 类型 [%s] 执行失败: %v", task.Name, task.ID, task.Type, err)
				// 异常完成：发送完成通知，不发送回调
				if err2 := common.NotifyTaskComplete(task); err2 != nil {
					common.Logger.Errorf("发送任务完成通知失败: %v", err2)
				}

				// 发送飞书完成通知
				if err2 := common.SendFeishuTaskCompleteNotification(task); err2 != nil {
					common.Logger.Errorf("发送飞书任务完成通知失败: %v", err2)
				}
				return // 异常情况不发送回调
			}
		} else {
			task.Status = models.TaskStatusSuccess
			task.Result = "执行成功"
			common.Logger.Infof("任务 %s [%s] 类型 [%s] 执行成功", task.Name, task.ID, task.Type)
			// 正常完成：发送回调，不发送完成通知（由agent处理最终通知）
			if err := common.SendTaskCallback(task); err != nil {
				common.Logger.Errorf("发送任务回调失败: %v", err)
			}
		}
	}()
}

// checkGitCloneSuccess 检查Git克隆是否成功
func CheckGitCloneSuccess(gitCloneDir string) error {
	// 检查目录是否存在
	if _, err := os.Stat(gitCloneDir); os.IsNotExist(err) {
		return fmt.Errorf("Git克隆目录不存在: %s", gitCloneDir)
	}

	// 检查是否为空目录
	files, err := os.ReadDir(gitCloneDir)
	if err != nil {
		return fmt.Errorf("读取Git克隆目录失败: %v", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("Git克隆目录为空")
	}

	// 检查是否包含.git目录或基本文件
	hasContent := false
	for _, file := range files {
		if file.Name() == ".git" || file.Name() == "pom.xml" || file.Name() == "package.json" || file.Name() == "README.md" {
			hasContent = true
			break
		}
	}

	if !hasContent {
		return fmt.Errorf("Git克隆目录缺少项目文件")
	}

	return nil
}

// ExecuteCommandWithLog 执行命令并输出到指定日志文件，支持取消信号
func ExecuteCommandWithLog(task *models.Task, command, logFile string) error {
	cmd := exec.Command("bash", "-c", command)

	// 创建日志文件
	file, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %v", err)
	}
	defer file.Close()

	// 设置输出到文件
	cmd.Stdout = file
	cmd.Stderr = file

	// 启动命令
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动命令失败: %v", err)
	}

	// 创建一个channel来接收命令完成信号
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// 等待命令完成或取消信号
	select {
	case <-task.CancelChan:
		// 收到取消信号，清理Docker容器并终止进程
		CleanupDockerContainers(task, command)
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return fmt.Errorf("命令被取消")
	case err := <-done:
		// 命令正常完成
		if err != nil {
			return fmt.Errorf("命令执行失败: %v", err)
		}
		return nil
	}
}

// ExecuteCommand 执行Shell命令并记录输出
func ExecuteCommand(task *models.Task, command string) error {
	// 创建一个bash命令
	cmd := exec.Command("bash", "-c", command)

	// 创建管道获取命令输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		return err
	}

	// 创建一个完成通道
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// 读取标准输出和错误输出（不再保存到内存）
	go io.Copy(io.Discard, stdout)
	go io.Copy(io.Discard, stderr)

	// 等待命令完成或取消
	select {
	case <-task.CancelChan:
		// 尝试终止进程
		if err := cmd.Process.Kill(); err != nil {
			LogToConsole(task, fmt.Sprintf("无法终止进程: %v", err))
		}
		return errors.New("任务被取消")
	case err := <-done:
		return err
	}
}

// CleanupDockerContainers 清理与任务相关的Docker容器
func CleanupDockerContainers(task *models.Task, command string) {
	// 检查命令是否包含docker run
	if !strings.Contains(command, "docker run") {
		return
	}

	LogToConsole(task, "任务取消，开始清理Docker容器...")

	// 方法1: 根据任务ID查找并停止容器
	// 查找可能与当前任务相关的容器（基于镜像名称）
	images := []string{
		"prohub.hzbxhd.com/library/git-clone",
		"prohub.hzbxhd.com/library/maven-java8",
		"prohub.hzbxhd.com/library/maven-java11",
		"prohub.hzbxhd.com/library/maven-java17",
	}

	for _, image := range images {
		if strings.Contains(command, image) {
			// 查找使用该镜像的运行中容器
			psCmd := fmt.Sprintf("docker ps --filter ancestor=%s --format \"{{.ID}}\"", image)
			cmd := exec.Command("bash", "-c", psCmd)
			output, err := cmd.Output()
			if err != nil {
				LogToConsole(task, fmt.Sprintf("查找容器失败: %v", err))
				continue
			}

			containerIDs := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, containerID := range containerIDs {
				if containerID != "" {
					// 停止容器
					stopCmd := fmt.Sprintf("docker stop %s", containerID)
					stopExec := exec.Command("bash", "-c", stopCmd)
					if err := stopExec.Run(); err != nil {
						LogToConsole(task, fmt.Sprintf("停止容器 %s 失败: %v", containerID, err))
					} else {
						LogToConsole(task, fmt.Sprintf("成功停止容器: %s", containerID))
					}

					// 删除容器
					rmCmd := fmt.Sprintf("docker rm %s", containerID)
					rmExec := exec.Command("bash", "-c", rmCmd)
					if err := rmExec.Run(); err != nil {
						LogToConsole(task, fmt.Sprintf("删除容器 %s 失败: %v", containerID, err))
					} else {
						LogToConsole(task, fmt.Sprintf("成功删除容器: %s", containerID))
					}
				}
			}
			break
		}
	}

	// 方法2: 强制清理所有可能的孤儿容器
	// 清理可能的孤儿容器（运行时间较短的容器，可能是刚启动的）
	cleanupCmd := "docker ps --filter status=running --format \"{{.ID}} {{.Image}} {{.RunningFor}}\" | grep -E \"(git-clone|maven)\" | awk '{if($3 ~ /second/ || $3 ~ /minute/) print $1}' | xargs -r docker stop"
	cleanupExec := exec.Command("bash", "-c", cleanupCmd)
	if err := cleanupExec.Run(); err != nil {
		LogToConsole(task, fmt.Sprintf("清理孤儿容器失败: %v", err))
	} else {
		LogToConsole(task, "完成孤儿容器清理")
	}
}

// ExecuteTask 执行具体任务
func ExecuteTask(task *models.Task, projectConfig *models.ProjectConfig) error {
	// 根据任务类型执行不同的任务：web执行前端构建，其他执行后端构建
	if task.Type == "web" {
		LogToConsole(task, "执行Web任务...")
		return ExecuteWebBuildTask(task, projectConfig)
	}
	LogToConsole(task, "执行后端任务...")
	return ExecuteJavaBuildTask(task, projectConfig)
}

// appendToStepLogFile 向步骤日志文件追加内容
func appendToStepLogFile(filePath string, content string) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(content + "\n"); err != nil {
		return err
	}

	return nil
}

// AppendToFile 向文件追加内容（通用工具函数）
func AppendToFile(filePath string, content string) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(content + "\n"); err != nil {
		return err
	}

	return nil
}

// AddStepLog 添加步骤日志（只写入工作目录步骤日志文件）
func AddStepLog(task *models.Task, stepType string, logMessage string) {
	logEntry := time.Now().Format("2006-01-02 15:04:05") + " " + logMessage

	// 只写入工作目录的步骤日志文件
	if stepType != "" && task.LogDir != "" {
		stepLogPath := filepath.Join(task.LogDir, stepType+".log")
		if err := appendToStepLogFile(stepLogPath, logEntry); err != nil {
			// 写入失败输出到控制台（服务日志）
			common.Logger.Errorf("[任务%s][步骤%s] 写入步骤日志失败: %v", task.ID, stepType, err)
		}
	}
}

// LogToConsole 输出服务日志到控制台
func LogToConsole(task *models.Task, logMessage string) {
	common.Logger.Infof("[任务%s] %s", task.ID, logMessage)
}

// CreateStepLogFunc 创建一个步骤日志函数（闭包）
func CreateStepLogFunc(task *models.Task, stepName string) func(*models.Task, string) {
	return func(t *models.Task, logMessage string) {
		AddStepLog(t, stepName, logMessage)
	}
}
