package getProduct

import (
	"cicd-server/models"
	"fmt"
	"os"
	"path/filepath"
)

// ExecuteRiskProduct 执行Risk项目产物获取
func ExecuteRiskProduct(task *models.Task, projectName, gitCloneDir, productDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error) error {

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	addTaskLog(task, "开始获取Risk项目产物")

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

	// 3. 处理Risk项目的modules目录
	return processRiskModules(task, projectName, gitCloneDir, productDir, addTaskLog)
}

// processRiskModules 处理Risk项目的modules目录
func processRiskModules(task *models.Task, projectName, gitCloneDir, productDir string,
	addTaskLog func(*models.Task, string)) error {

	// Risk项目的modules目录路径
	modulesDir := filepath.Join(gitCloneDir, "bxhd-risk-modules")

	// 检查modules目录是否存在
	if _, err := os.Stat(modulesDir); os.IsNotExist(err) {
		return fmt.Errorf("bxhd-risk-modules目录不存在: %s", modulesDir)
	}

	addTaskLog(task, "开始处理Risk项目modules目录")

	// 删除不需要的文件
	pomFile := filepath.Join(modulesDir, "pom.xml")
	if err := os.Remove(pomFile); err != nil && !os.IsNotExist(err) {
		addTaskLog(task, fmt.Sprintf("删除pom.xml失败: %v", err))
	} else {
		addTaskLog(task, "删除pom.xml完成")
	}

	// 删除不需要的目录
	dirsToRemove := []string{
		filepath.Join(modulesDir, projectName+"-push"),
		filepath.Join(modulesDir, projectName+"-bjzy-redis"),
		filepath.Join(modulesDir, projectName+"-gen"),
		filepath.Join(modulesDir, projectName+"-pay"),
		filepath.Join(modulesDir, projectName+"-risk"),
		filepath.Join(modulesDir, projectName+"-record"),
	}

	for _, dir := range dirsToRemove {
		if err := os.RemoveAll(dir); err != nil {
			addTaskLog(task, fmt.Sprintf("删除目录 %s 失败: %v", filepath.Base(dir), err))
		} else {
			addTaskLog(task, fmt.Sprintf("删除目录 %s 完成", filepath.Base(dir)))
		}
	}

	// 移动剩余的modules内容到产物目录
	if err := moveModulesContents(modulesDir, productDir); err != nil {
		addTaskLog(task, fmt.Sprintf("移动modules内容失败: %v", err))
		return fmt.Errorf("移动modules内容失败: %v", err)
	}

	addTaskLog(task, "Risk项目产物获取完成")
	return nil
}

// moveModulesContents 移动modules目录中的内容到产物目录
func moveModulesContents(modulesDir, productDir string) error {
	// 读取modules目录中的所有内容
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return fmt.Errorf("读取modules目录失败: %v", err)
	}

	// 移动每个子目录/文件到产物目录
	for _, entry := range entries {
		// 过滤掉隐藏文件（以.开头的文件/目录）
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}

		srcPath := filepath.Join(modulesDir, entry.Name())
		dstPath := filepath.Join(productDir, entry.Name())

		if err := os.Rename(srcPath, dstPath); err != nil {
			return fmt.Errorf("移动 %s 到产物目录失败: %v", entry.Name(), err)
		}
	}

	return nil
}
