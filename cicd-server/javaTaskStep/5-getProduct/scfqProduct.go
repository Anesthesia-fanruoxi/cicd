package getProduct

import (
	"cicd-server/models"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExecuteScfqProduct 执行SCFQ项目产物获取
func ExecuteScfqProduct(task *models.Task, projectName, gitCloneDir, productDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error) error {

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	addTaskLog(task, "开始获取SCFQ项目产物")

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

	// 3. 处理SCFQ项目产物
	return processScfqProduct(task, projectName, gitCloneDir, productDir, addTaskLog)
}

// processScfqProduct 处理SCFQ项目产物
func processScfqProduct(task *models.Task, projectName, gitCloneDir, productDir string,
	addTaskLog func(*models.Task, string)) error {

	addTaskLog(task, "开始清理不需要的文件和目录")

	// 定义需要删除的文件
	filesToRemove := []string{
		filepath.Join(gitCloneDir, "LICENSE"),
		filepath.Join(gitCloneDir, "README.md"),
		filepath.Join(gitCloneDir, "deploy-api.yml"),
		filepath.Join(gitCloneDir, "docker-image.sh"),
		filepath.Join(gitCloneDir, "pom.xml"),
		filepath.Join(gitCloneDir, "pushGithub.sh"),
	}

	// 定义需要删除的目录
	dirsToRemove := []string{
		filepath.Join(gitCloneDir, "DB"),
		filepath.Join(gitCloneDir, "admin"),
		filepath.Join(gitCloneDir, "scfq-admin"),
		filepath.Join(gitCloneDir, "scfq-im-api"),
		filepath.Join(gitCloneDir, "config"),
		filepath.Join(gitCloneDir, "docs"),
		filepath.Join(gitCloneDir, "framework"),
		filepath.Join(gitCloneDir, "xxl-job"),
	}

	// 删除文件
	for _, file := range filesToRemove {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			addTaskLog(task, fmt.Sprintf("删除文件 %s 失败: %v", filepath.Base(file), err))
		} else if err == nil {
			addTaskLog(task, fmt.Sprintf("删除文件 %s 完成", filepath.Base(file)))
		}
	}

	// 删除目录
	for _, dir := range dirsToRemove {
		if err := os.RemoveAll(dir); err != nil {
			addTaskLog(task, fmt.Sprintf("删除目录 %s 失败: %v", filepath.Base(dir), err))
		} else {
			addTaskLog(task, fmt.Sprintf("删除目录 %s 完成", filepath.Base(dir)))
		}
	}

	addTaskLog(task, "清理完成，开始移动剩余产物")

	// 移动剩余内容到产物目录
	if err := moveScfqContents(gitCloneDir, productDir); err != nil {
		addTaskLog(task, fmt.Sprintf("移动SCFQ产物失败: %v", err))
		return fmt.Errorf("移动SCFQ产物失败: %v", err)
	}

	addTaskLog(task, "开始创建标准pkg结构")

	// 为每个子项目创建标准的pkg结构
	if err := createStandardPkgStructure(task, productDir, addTaskLog); err != nil {
		addTaskLog(task, fmt.Sprintf("创建标准pkg结构失败: %v", err))
		return fmt.Errorf("创建标准pkg结构失败: %v", err)
	}

	addTaskLog(task, "SCFQ项目产物获取完成")
	return nil
}

// moveScfqContents 移动gitCloneDir中的剩余内容到产物目录
func moveScfqContents(gitCloneDir, productDir string) error {
	// 读取gitCloneDir目录中的所有内容
	entries, err := os.ReadDir(gitCloneDir)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %v", err)
	}

	// 移动每个子目录/文件到产物目录
	for _, entry := range entries {
		// 过滤掉隐藏文件（以.开头的文件/目录）
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}

		srcPath := filepath.Join(gitCloneDir, entry.Name())
		dstPath := filepath.Join(productDir, entry.Name())

		if err := os.Rename(srcPath, dstPath); err != nil {
			return fmt.Errorf("移动 %s 到产物目录失败: %v", entry.Name(), err)
		}
	}

	return nil
}

// createStandardPkgStructure 为每个子项目创建标准的pkg结构
func createStandardPkgStructure(task *models.Task, productDir string, addTaskLog func(*models.Task, string)) error {
	// 读取产物目录中的所有子项目
	entries, err := os.ReadDir(productDir)
	if err != nil {
		return fmt.Errorf("读取产物目录失败: %v", err)
	}

	// 遍历每个子项目
	for _, entry := range entries {
		// 跳过非目录项
		if !entry.IsDir() {
			continue
		}

		subProjectName := entry.Name()
		subProjectPath := filepath.Join(productDir, subProjectName)
		targetPath := filepath.Join(subProjectPath, "target")

		// 检查target目录是否存在
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			continue
		}

		addTaskLog(task, fmt.Sprintf("处理子项目: %s", subProjectName))

		// 创建target/pkg目录
		pkgPath := filepath.Join(targetPath, "pkg")
		if err := os.MkdirAll(pkgPath, 0755); err != nil {
			return fmt.Errorf("创建pkg目录失败 %s: %v", subProjectName, err)
		}

		// 查找target目录下的jar文件（排除.original结尾的）
		targetEntries, err := os.ReadDir(targetPath)
		if err != nil {
			return fmt.Errorf("读取target目录失败 %s: %v", subProjectName, err)
		}

		for _, targetEntry := range targetEntries {
			// 只处理jar文件，跳过.original文件
			if !targetEntry.IsDir() && filepath.Ext(targetEntry.Name()) == ".jar" &&
				!strings.HasSuffix(targetEntry.Name(), ".original") {

				jarName := targetEntry.Name()
				jarSrcPath := filepath.Join(targetPath, jarName)
				jarDstPath := filepath.Join(pkgPath, jarName)

				// 移动jar文件到pkg目录
				if err := os.Rename(jarSrcPath, jarDstPath); err != nil {
					return fmt.Errorf("移动jar文件失败 %s: %v", jarName, err)
				}

				addTaskLog(task, fmt.Sprintf("  - 移动jar文件: %s -> pkg/%s", jarName, jarName))
			}
		}
	}

	return nil
}
