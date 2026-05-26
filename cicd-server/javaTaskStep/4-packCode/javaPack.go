package packCode

import (
	"bufio"
	"cicd-server/models"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExecuteJavaMavenPack 执行Java Maven编译打包
func ExecuteJavaMavenPack(task *models.Task, javaVersion, gitCloneDir, logDir, timeDir, taskLogDir string, addTaskLog func(*models.Task, string), executeCommand func(*models.Task, string) error, executeCommandWithLog func(*models.Task, string, string) error, appendToFile func(string, string) error) error {
	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
		// 继续执行
	}

	// 4. 编译代码
	addTaskLog(task, "开始编译代码")

	// 根据Java版本选择对应的Maven镜像
	var mvnImage string
	switch javaVersion {
	case "java17":
		mvnImage = "prohub.hzbxhd.com/library/java17-maven:1.0"
	case "java21":
		mvnImage = "prohub.hzbxhd.com/library/java21-maven:1.0"
	case "java8":
		mvnImage = "prohub.hzbxhd.com/library/java8-maven:1.0"
	default:
		mvnImage = "prohub.hzbxhd.com/library/java17-maven:1.0" // 默认使用java17
		addTaskLog(task, fmt.Sprintf("未知的Java版本: %s，使用默认的java17", javaVersion))
	}

	addTaskLog(task, fmt.Sprintf("使用Java版本: %s，镜像: %s", javaVersion, mvnImage))
	mvnLogFile := filepath.Join(taskLogDir, "mvn.log")
	mvnCmd := fmt.Sprintf("docker run --rm -v %s:/app -v ~/.m2/repository:/root/.m2/repository %s /bin/sh -c \"cd /app && mvn -B -T 4 clean package -Dmaven.test.skip=true -Dautoconfig.skip\"",
		gitCloneDir, mvnImage)

	mavenStartTime := time.Now()
	addTaskLog(task, fmt.Sprintf("执行命令: %s", mvnCmd))
	addTaskLog(task, fmt.Sprintf("Maven日志输出到: %s", mvnLogFile))

	if err := executeCommandWithLog(task, mvnCmd, mvnLogFile); err != nil {
		addTaskLog(task, fmt.Sprintf("Maven编译失败: %v", err))

		// 提取Maven错误日志
		mavenErrorFile := filepath.Join(taskLogDir, "mvn_error.log")
		grepCmd := fmt.Sprintf("grep \"ERROR\" %s > %s", mvnLogFile, mavenErrorFile)
		if err := executeCommand(task, grepCmd); err != nil {
			addTaskLog(task, fmt.Sprintf("提取Maven错误日志失败: %v", err))
		}

		return err
	}

	// 检查Maven编译是否成功
	if err := checkMavenBuildSuccess(gitCloneDir, mvnLogFile); err != nil {
		addTaskLog(task, fmt.Sprintf("Maven编译验证失败: %v", err))

		// 提取Maven错误日志
		mavenErrorFile := filepath.Join(taskLogDir, "mvn_error.log")
		grepCmd := fmt.Sprintf("grep -E \"ERROR|FAILED|BUILD FAILURE\" %s > %s", mvnLogFile, mavenErrorFile)
		if err := executeCommand(task, grepCmd); err != nil {
			addTaskLog(task, fmt.Sprintf("提取Maven错误日志失败: %v", err))
		}

		return err
	}
	addTaskLog(task, "Maven编译验证成功")

	mavenEndTime := time.Now()
	mavenDuration := mavenEndTime.Sub(mavenStartTime).Seconds()
	addTaskLog(task, fmt.Sprintf("代码编译完成，耗时: %.2f秒", mavenDuration))

	// 记录Maven编译时间
	if err := appendToFile(filepath.Join(timeDir, "ready_time.txt"),
		fmt.Sprintf("%.0f", mavenDuration)); err != nil {
		addTaskLog(task, fmt.Sprintf("写入Maven编译时间失败: %v", err))
	}

	return nil
}

// checkMavenBuildSuccess 检查Maven编译是否成功
func checkMavenBuildSuccess(gitCloneDir, mvnLogFile string) error {
	// 检查Maven日志文件是否存在
	if _, err := os.Stat(mvnLogFile); os.IsNotExist(err) {
		return fmt.Errorf("Maven日志文件不存在: %s", mvnLogFile)
	}

	// 读取Maven日志并检查编译结果
	file, err := os.Open(mvnLogFile)
	if err != nil {
		return fmt.Errorf("读取Maven日志失败: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buildSuccess := false
	buildFailure := false
	hasErrors := false

	for scanner.Scan() {
		line := strings.ToUpper(scanner.Text()) // 转为大写进行匹配，提高匹配准确性

		// 检查成功标识
		if strings.Contains(line, "BUILD SUCCESS") {
			buildSuccess = true
		}

		// 检查失败标识
		if strings.Contains(line, "BUILD FAILURE") ||
			strings.Contains(line, "BUILD FAILED") ||
			strings.Contains(line, "COMPILATION ERROR") ||
			strings.Contains(line, "FATAL ERROR") {
			buildFailure = true
		}

		// 检查错误信息
		if strings.Contains(line, "[ERROR]") &&
			!strings.Contains(line, "DOWNLOADING") &&
			!strings.Contains(line, "DOWNLOADED") {
			hasErrors = true
		}
	}

	// 优先检查明确的失败标识
	if buildFailure {
		return fmt.Errorf("Maven编译失败: 日志中发现BUILD FAILURE")
	}

	// 检查是否有错误信息
	if hasErrors && !buildSuccess {
		return fmt.Errorf("Maven编译失败: 日志中发现ERROR信息且无BUILD SUCCESS")
	}

	// 检查是否有成功标识
	if !buildSuccess {
		return fmt.Errorf("Maven编译失败: 日志中未找到BUILD SUCCESS")
	}

	return nil
}
