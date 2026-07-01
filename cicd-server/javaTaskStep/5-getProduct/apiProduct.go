package getProduct

import (
	"cicd-server/config"
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

	// 从配置文件获取产品类型
	productType, exists := config.GetProductType(projectName)
	if !exists {
		return fmt.Errorf("项目 %s 未配置产品类型，请在 config/product_rules.json 中添加", projectName)
	}

	// 根据产品类型路由到对应的处理函数
	switch productType {
	case "default":
		return ExecuteDefaultProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	case "risk":
		return ExecuteRiskProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	case "dh":
		return ExecuteDhProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	case "crm":
		return ExecuteCrmProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	case "scfq":
		return ExecuteScfqProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	case "bigdata":
		return ExecuteBigDataProduct(task, projectName, gitCloneDir, productDir, taskLogDir, addTaskLog, executeCommand)
	default:
		return fmt.Errorf("项目 %s 的产品类型 %s 未注册处理函数，请检查代码", projectName, productType)
	}
}
