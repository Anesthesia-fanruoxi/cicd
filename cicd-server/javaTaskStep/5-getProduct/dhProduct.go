package getProduct

import (
	"cicd-server/models"
	"fmt"
	"os"
	"path/filepath"
)

// ExecuteDhProduct 执行DH（贷后）项目产物获取
func ExecuteDhProduct(task *models.Task, projectName, gitCloneDir, productDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error) error {

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	addTaskLog(task, "开始获取DH项目产物")

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

	// 3. 处理DH项目的产物
	return processDhProduct(task, projectName, gitCloneDir, productDir, addTaskLog)
}

// processDhProduct 处理DH项目产物
func processDhProduct(task *models.Task, projectName, gitCloneDir, productDir string,
	addTaskLog func(*models.Task, string)) error {

	// DH项目的jar文件路径: jxh-pl-bi-starter/target/jxh-pl-bi-starter.jar
	jarSourcePath := filepath.Join(gitCloneDir, "jxh-pl-bi-starter", "target", "jxh-pl-bi-starter.jar")

	// 检查jar文件是否存在
	if _, err := os.Stat(jarSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("DH项目jar包不存在: %s", jarSourcePath)
	}

	addTaskLog(task, fmt.Sprintf("发现DH项目jar包: %s", jarSourcePath))

	// 创建符合标准结构的目录：jxh-pl-bi-starter/target/pkg/
	targetPkgDir := filepath.Join(productDir, "jxh-pl-bi-starter", "target", "pkg")
	if err := os.MkdirAll(targetPkgDir, 0755); err != nil {
		return fmt.Errorf("创建target/pkg目录失败: %v", err)
	}

	addTaskLog(task, fmt.Sprintf("创建标准产物目录: %s", targetPkgDir))

	// 目标jar文件路径（放在pkg目录下）
	jarDestPath := filepath.Join(targetPkgDir, "jxh-pl-bi-starter.jar")

	// 复制jar文件到标准目录
	if err := copyFile(jarSourcePath, jarDestPath); err != nil {
		addTaskLog(task, fmt.Sprintf("复制jar包失败: %v", err))
		return fmt.Errorf("复制jar包失败: %v", err)
	}

	addTaskLog(task, fmt.Sprintf("jar包已复制到: %s", jarDestPath))
	addTaskLog(task, "DH项目产物获取完成")
	return nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	// 读取源文件
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("读取源文件失败: %v", err)
	}

	// 写入目标文件
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("写入目标文件失败: %v", err)
	}

	return nil
}
