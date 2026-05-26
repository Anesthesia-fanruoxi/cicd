package api

import (
	"cicd-server/common"
	"cicd-server/models"
	"cicd-server/taskBuilder"
	"encoding/json"
	"net/http"
	"time"
)

// 格式化时间为字符串，处理空指针
func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// CreateTask 创建任务
func CreateTask(w http.ResponseWriter, r *http.Request) {
	// 只接受POST请求
	if r.Method != http.MethodPost {
		responseJSON(w, common.ErrorResponse(405, "方法不允许"), http.StatusMethodNotAllowed)
		return
	}

	// 解析请求体
	var req models.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		responseJSON(w, common.ErrorResponse(400, "无效的请求参数"), http.StatusBadRequest)
		return
	}

	// 验证参数
	if req.Project == "" {
		responseJSON(w, common.ErrorResponse(400, "项目名称不能为空"), http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		responseJSON(w, common.ErrorResponse(400, "任务类型不能为空"), http.StatusBadRequest)
		return
	}
	// type: "web"->web池, "double"/"single"等->default池

	// 创建任务
	task, err := taskBuilder.CreateTask(req.Project, req.Type, req.CallbackURL, req.Category, req.Description, req.CreatedBy, req.CreatedByName)
	if err != nil {
		responseJSON(w, common.ErrorResponse(500, "创建任务失败: "+err.Error()), http.StatusInternalServerError)
		return
	}

	// 返回任务信息
	responseJSON(w, common.SuccessResponse(map[string]interface{}{
		"id":      task.ID,
		"type":    req.Type,
		"project": req.Project,
	}), http.StatusOK)
}

// GetTaskList 获取任务列表
func GetTaskList(w http.ResponseWriter, r *http.Request) {
	// 只接受GET请求
	if r.Method != http.MethodGet {
		responseJSON(w, common.ErrorResponse(405, "方法不允许"), http.StatusMethodNotAllowed)
		return
	}

	// 获取任务类型参数
	taskType := r.URL.Query().Get("type")

	var tasks []*models.Task

	// 根据类型参数获取任务列表
	if taskType != "" {
		tasks, _ = taskBuilder.GetTaskListByType(taskType)
	} else {
		// 获取所有任务列表
		tasks = taskBuilder.GetTaskList()
	}

	// 构造响应数据
	taskList := make([]models.TaskListItem, 0, len(tasks))
	for _, task := range tasks {
		taskList = append(taskList, models.TaskListItem{
			ID:         task.ID,
			Name:       task.Name,
			Type:       task.Type,
			Status:     task.Status,
			CreatedAt:  task.CreatedAt.Format("2006-01-02 15:04:05"),
			StartedAt:  formatTimePtr(task.StartedAt),
			FinishedAt: formatTimePtr(task.FinishedAt),
		})
	}

	responseJSON(w, common.SuccessResponse(taskList), http.StatusOK)
}

// GetTaskDetail 获取任务详情
func GetTaskDetail(w http.ResponseWriter, r *http.Request) {
	// 只接受GET请求
	if r.Method != http.MethodGet {
		responseJSON(w, common.ErrorResponse(405, "方法不允许"), http.StatusMethodNotAllowed)
		return
	}

	// 获取任务ID参数
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		responseJSON(w, common.ErrorResponse(400, "缺少任务ID参数"), http.StatusBadRequest)
		return
	}

	// 获取任务详情
	task, err := taskBuilder.GetTaskByID(taskID)
	if err != nil {
		responseJSON(w, common.ErrorResponse(404, err.Error()), http.StatusNotFound)
		return
	}

	// 构造任务详情响应
	detail := models.TaskDetail{
		ID:         task.ID,
		Name:       task.Name,
		Type:       task.Type,
		Status:     task.Status,
		CreatedAt:  task.CreatedAt.Format("2006-01-02 15:04:05"),
		StartedAt:  formatTimePtr(task.StartedAt),
		FinishedAt: formatTimePtr(task.FinishedAt),
		Result:     task.Result,
	}

	// 如果有项目信息，直接将其字段添加到响应中
	if task.ProjectInfo != nil {
		// 根据任务类型选择Git仓库
		if task.Type == "web" {
			detail.GitURL = task.ProjectInfo.GitVue
		} else {
			detail.GitURL = task.ProjectInfo.GitBackend
		}
		detail.Description = task.ProjectInfo.Description
		detail.UpdateFeishu = task.ProjectInfo.UpdateFeishu
		detail.NotifyFeishu = task.ProjectInfo.NotifyFeishu
	}

	responseJSON(w, common.SuccessResponse(detail), http.StatusOK)
}

// CancelTask 取消任务
func CancelTask(w http.ResponseWriter, r *http.Request) {
	// 只接受POST请求
	if r.Method != http.MethodPost {
		responseJSON(w, common.ErrorResponse(405, "方法不允许"), http.StatusMethodNotAllowed)
		return
	}

	// 解析加密的请求体
	var encryptedReq struct {
		Data string `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&encryptedReq); err != nil {
		responseJSON(w, common.ErrorResponse(400, "无效的请求参数"), http.StatusBadRequest)
		return
	}

	// 解密请求数据
	decryptedData, err := common.DecryptAndDecompress(encryptedReq.Data)
	if err != nil {
		responseJSON(w, common.ErrorResponse(400, "解密请求失败"), http.StatusBadRequest)
		return
	}

	// 解析解密后的数据
	var req struct {
		ID string `json:"id"`
	}

	if err := json.Unmarshal(decryptedData, &req); err != nil {
		responseJSON(w, common.ErrorResponse(400, "解析请求数据失败"), http.StatusBadRequest)
		return
	}

	// 验证参数
	if req.ID == "" {
		responseJSON(w, common.ErrorResponse(400, "任务ID不能为空"), http.StatusBadRequest)
		return
	}

	// 取消任务
	err = taskBuilder.CancelTask(req.ID)
	if err != nil {
		responseJSON(w, common.ErrorResponse(400, err.Error()), http.StatusBadRequest)
		return
	}

	responseJSON(w, common.SuccessResponse(nil), http.StatusOK)
}

// 将响应序列化为JSON并写入ResponseWriter
func responseJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	// 序列化响应数据
	jsonData, err := json.Marshal(data)
	if err != nil {
		common.Logger.Errorf("序列化响应数据失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// 加密响应数据
	encryptedData, err := common.CompressAndEncrypt(jsonData)
	if err != nil {
		common.Logger.Errorf("加密响应数据失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// 构建加密响应
	encryptedResp := common.EncryptedResponse{
		Code: 200,
		Msg:  "success",
		Data: encryptedData,
	}

	// 设置内容类型
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// 序列化并写入加密响应
	if err := json.NewEncoder(w).Encode(encryptedResp); err != nil {
		common.Logger.Errorf("响应写入失败: %v", err)
	}
}
