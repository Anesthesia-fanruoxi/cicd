package router

import (
	"cicd-server/api"
	"cicd-server/common"
	"net/http"
)

// InitRouter 初始化路由
func InitRouter() *http.ServeMux {
	mux := http.NewServeMux()

	// 任务管理接口
	mux.HandleFunc("/api/task/create", api.CreateTask)
	mux.HandleFunc("/api/task/list", api.GetTaskList)
	mux.HandleFunc("/api/task/detail", api.GetTaskDetail)
	mux.HandleFunc("/api/task/cancel", api.CancelTask)

	// 项目配置管理接口（CMDB推送）
	mux.HandleFunc("/api/config/update", api.UpdateProjectConfig)
	mux.HandleFunc("/api/config/batch-update", api.BatchUpdateProjectConfig)

	// WebSocket接口
	mux.HandleFunc("/ws/cicd/logs", api.TaskLogWebSocket)

	common.Logger.Info("API路由注册完成")
	return mux
}
