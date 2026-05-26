package getProduct

import (
	"cicd-server/models"
	"fmt"
	"os"
	"path/filepath"
)

// ExecuteCrmProduct 执行DH（贷后）项目产物获取
func ExecuteCrmProduct(task *models.Task, projectName, gitCloneDir, productDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error) error {

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	addTaskLog(task, "开始获取CRM项目产物")

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

	// 3. 处理CRM项目的产物
	return processCrmProduct(task, projectName, gitCloneDir, productDir, addTaskLog)
}

// processCrmProduct 处理CRM项目产物
func processCrmProduct(task *models.Task, projectName, gitCloneDir, productDir string,
	addTaskLog func(*models.Task, string)) error {

	// CRM项目的jar文件路径: target/telemarketing-admin.jar
	jarSourcePath := filepath.Join(gitCloneDir, "target", "telemarketing-admin.jar")

	// 检查jar文件是否存在
	if _, err := os.Stat(jarSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("CRM项目jar包不存在: %s", jarSourcePath)
	}

	addTaskLog(task, fmt.Sprintf("发现CRM项目jar包: %s", jarSourcePath))

	// 创建符合标准结构的目录：telemarketing-admin/target/pkg/
	targetPkgDir := filepath.Join(productDir, "telemarketing-admin", "target", "pkg")
	if err := os.MkdirAll(targetPkgDir, 0755); err != nil {
		return fmt.Errorf("创建target/pkg目录失败: %v", err)
	}

	addTaskLog(task, fmt.Sprintf("创建标准产物目录: %s", targetPkgDir))

	// 目标jar文件路径（放在pkg目录下）
	jarDestPath := filepath.Join(targetPkgDir, "telemarketing-admin.jar")

	// 复制jar文件到标准目录
	if err := copyFile(jarSourcePath, jarDestPath); err != nil {
		addTaskLog(task, fmt.Sprintf("复制jar包失败: %v", err))
		return fmt.Errorf("复制jar包失败: %v", err)
	}

	addTaskLog(task, fmt.Sprintf("jar包已复制到: %s", jarDestPath))
	addTaskLog(task, "CRM项目产物获取完成")
	return nil
}
