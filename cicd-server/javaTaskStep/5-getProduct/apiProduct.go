package getProduct

import (
	"cicd-server/models"
	"fmt"
)

// ExecuteProductAPI 统一的产物获取API入口
func ExecuteProductAPI(task *models.Task, projectName, gitCloneDir, productDir, taskLogDir string,
	addTaskLog func(*models.Task, string),
	executeCommand func(*models.Task, string) error) error {

	// 检查任务是否被取消
	select {
	case <-task.CancelChan:
		addTaskLog(task, "任务被取消")
		return fmt.Errorf("任务被取消")
	default:
	}

	addTaskLog(task, fmt.Sprintf("开始获取项目 %s 的产物", projectName))

	// 根据项目名称路由到对应的处理函数
	switch projectName {
	case "jxh", "axh", "bjjf", "ysh", "mhg":
		// 默认Java项目类型
		return ExecuteDefaultProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	case "jxh-risk", "ysh-risk":
		// Risk项目类型
		return ExecuteRiskProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	case "jxh-dh":
		// 贷后项目类型
		return ExecuteDhProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	case "bjjf-crm", "axh-crm":
		// CRM项目类型
		return ExecuteCrmProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	case "scfq":
		return ExecuteScfqProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	case "jzsk":
		// 大数据项目类型
		return ExecuteBigDataProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	default:
		return fmt.Errorf("暂不支持项目: %s", projectName)
	}
}
