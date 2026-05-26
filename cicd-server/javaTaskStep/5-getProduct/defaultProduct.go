package getProduct

import (
	"cicd-server/models"
	"fmt"
	"os"
	"path/filepath"
)

// ExecuteDefaultProduct 执行默认项目产物获取
func ExecuteDefaultProduct(task *models.Task, projectName, gitCloneDir, productDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error) error {

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	addTaskLog(task, "开始获取默认项目产物")

	// 1. 清理产物目录
	if err := os.RemoveAll(productDir); err != nil {
		addTaskLog(task, fmt.Sprintf("清理产物目录失败: %v", err))
		return fmt.Errorf("清理产物目录失败: %v", err)
	}
	addTaskLog(task, "产物目录清理完成")

	// 2. 创建产物目录
	if err := os.MkdirAll(productDir, 0755); err != nil {
		addTaskLog(task, fmt.Sprintf("创建产物目录失败: %v", err))
		return fmt.Errorf("创建产物目录失败: %v", err)
	}

	// 3. 获取默认项目产物
	return getDefaultProduct(task, projectName, gitCloneDir, productDir, addTaskLog, executeCommand)
}

// getDefaultProduct 获取默认项目产物
func getDefaultProduct(task *models.Task, projectName, gitCloneDir, productDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error) error {

	addTaskLog(task, "开始移动默认项目产物")

	// 移动 auth 模块
	authSrc := filepath.Join(gitCloneDir, projectName+"-auth")
	authDst := filepath.Join(productDir, projectName+"-auth")
	if err := moveDirectory(authSrc, authDst); err != nil {
		addTaskLog(task, fmt.Sprintf("移动auth模块失败: %v", err))
		return err
	}
	addTaskLog(task, "auth模块移动完成")

	// 移动 gateway 模块
	gatewaySrc := filepath.Join(gitCloneDir, projectName+"-gateway")
	gatewayDst := filepath.Join(productDir, projectName+"-gateway")
	if err := moveDirectory(gatewaySrc, gatewayDst); err != nil {
		addTaskLog(task, fmt.Sprintf("移动gateway模块失败: %v", err))
		return err
	}
	addTaskLog(task, "gateway模块移动完成")

	// 处理 modules 目录
	modulesDir := filepath.Join(gitCloneDir, projectName+"-modules")
	if err := processModulesDir(task, projectName, modulesDir, productDir, addTaskLog); err != nil {
		return err
	}

	addTaskLog(task, "默认项目产物获取完成")
	return nil
}

// processModulesDir 处理modules目录
func processModulesDir(task *models.Task, projectName, modulesDir, productDir string,
	addTaskLog func(*models.Task, string)) error {

	// 删除不需要的文件和目录
	filesToRemove := []string{
		filepath.Join(modulesDir, "pom.xml"),
	}

	dirsToRemove := []string{
		filepath.Join(modulesDir, projectName+"-push"),
		filepath.Join(modulesDir, projectName+"-bjzy-redis"),
		filepath.Join(modulesDir, projectName+"-gen"),
		filepath.Join(modulesDir, projectName+"-pay"),
		filepath.Join(modulesDir, projectName+"-risk"),
		filepath.Join(modulesDir, projectName+"-record"),
		filepath.Join(modulesDir, projectName+"-statistics"),
	}

	// 删除文件
	for _, file := range filesToRemove {
		if err := removeFileIfExists(file); err != nil {
			addTaskLog(task, fmt.Sprintf("删除文件失败 %s: %v", file, err))
			return err
		}
	}

	// 删除目录
	for _, dir := range dirsToRemove {
		if err := removeDirIfExists(dir); err != nil {
			addTaskLog(task, fmt.Sprintf("删除目录失败 %s: %v", dir, err))
			return err
		}
	}
	addTaskLog(task, "modules目录清理完成")

	// 移动剩余的modules内容到产物目录
	if err := moveModulesContent(modulesDir, productDir); err != nil {
		addTaskLog(task, fmt.Sprintf("移动modules内容失败: %v", err))
		return err
	}
	addTaskLog(task, "modules内容移动完成")

	return nil
}

// 辅助函数
func cleanProductDir(productDir string) error {
	if _, err := os.Stat(productDir); err == nil {
		return os.RemoveAll(productDir)
	}
	return nil
}

func moveDirectory(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("源目录不存在: %s", src)
	}
	return os.Rename(src, dst)
}

func removeFileIfExists(filePath string) error {
	if _, err := os.Stat(filePath); err == nil {
		return os.Remove(filePath)
	}
	return nil
}

func removeDirIfExists(dirPath string) error {
	if _, err := os.Stat(dirPath); err == nil {
		return os.RemoveAll(dirPath)
	}
	return nil
}

func moveModulesContent(modulesDir, productDir string) error {
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// 过滤掉隐藏文件（以.开头的文件/目录）
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}

		src := filepath.Join(modulesDir, entry.Name())
		dst := filepath.Join(productDir, entry.Name())
		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}
	return nil
}
