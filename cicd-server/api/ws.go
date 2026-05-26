package api

import (
	"bytes"
	"cicd-server/common"
	"cicd-server/taskBuilder"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 允许所有跨域请求
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// 任务日志WebSocket连接管理
type taskLogConnection struct {
	conn        *websocket.Conn
	taskID      string
	project     string
	stepType    string
	logFilePath string
	mu          sync.Mutex
	closeChan   chan struct{}
	lastFilePos int64
	logBuffer   []string     // 日志缓冲区
	bufferSize  int          // 缓冲区大小
	flushTicker *time.Ticker // 定时刷新缓冲区
	maxLines    int          // 最大发送行数
}

// TaskLogWebSocket 任务日志WebSocket处理函数
// 客户端示例代码：
// 1. 先加密参数: {taskId: "任务ID", projectName: "项目名称", stepType: "步骤名称"}
// 2. 使用加密后的data参数: const ws = new WebSocket(`ws://服务器地址/ws/task/logs?data=加密后的参数`);
//
//	ws.onmessage = function(event) {
//	  // event.data是纯文本日志，直接显示即可
//	  console.log(event.data);
//	};
//
//	ws.onclose = function() {
//	  console.log('任务可能已完成，WebSocket连接已关闭');
//	};
func TaskLogWebSocket(w http.ResponseWriter, r *http.Request) {
	// 获取加密的参数
	encryptedData := r.URL.Query().Get("data")
	if encryptedData == "" {
		http.Error(w, "缺少加密参数", http.StatusBadRequest)
		return
	}

	// 解密参数
	decryptedData, err := common.DecryptAndDecompress(encryptedData)
	if err != nil {
		http.Error(w, "解密参数失败", http.StatusBadRequest)
		return
	}

	// 解析解密后的参数
	var params struct {
		TaskID   string `json:"taskId"`
		Project  string `json:"project"`
		StepType string `json:"stepType"`
		Type     string `json:"type"`
	}

	if err := json.Unmarshal(decryptedData, &params); err != nil {
		http.Error(w, "解析参数失败", http.StatusBadRequest)
		return
	}

	taskID := params.TaskID
	project := params.Project
	stepType := params.StepType
	taskType := params.Type

	if taskID == "" {
		http.Error(w, "缺少任务ID参数", http.StatusBadRequest)
		return
	}
	if project == "" {
		http.Error(w, "缺少项目名称参数", http.StatusBadRequest)
		return
	}
	if stepType == "" {
		http.Error(w, "缺少步骤名称参数", http.StatusBadRequest)
		return
	}

	// 升级HTTP连接为WebSocket连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		common.Logger.Errorf("升级WebSocket连接失败: %v", err)
		return
	}

	// 构建日志文件路径
	logFilePath := buildLogFilePath(project, taskID, stepType, taskType)

	// 创建连接管理对象
	tc := &taskLogConnection{
		conn:        conn,
		taskID:      taskID,
		project:     project,
		stepType:    stepType,
		logFilePath: logFilePath,
		closeChan:   make(chan struct{}),
		lastFilePos: 0,
		logBuffer:   make([]string, 0, 100), // 初始化缓冲区，容量为100
		bufferSize:  0,
		flushTicker: time.NewTicker(200 * time.Millisecond), // 每200ms刷新一次缓冲区
		maxLines:    1000,                                   // 默认最多发送1000行
	}

	// 发送当前日志
	tc.sendCurrentLogs()

	// 启动监听任务日志的goroutine
	go tc.watchTaskLogs()

	// 启动缓冲区刷新goroutine
	go tc.flushBufferRoutine()

	// 处理客户端消息
	go tc.handleMessages()
}

// buildLogFilePath 构建日志文件路径
func buildLogFilePath(projectName, taskID, stepType, taskType string) string {
	// 根据步骤名称确定日志文件名
	var logFileName string
	switch stepType {
	case "git", "Git代码克隆":
		logFileName = "git.log"
	case "mvn", "编译代码", "maven":
		logFileName = "mvn.log"
	case "mvn_error", "编译错误":
		logFileName = "mvn_error.log"
	default:
		logFileName = stepType + ".log"
	}

	// 通过taskID获取任务信息，确定实际的项目目录名称
	actualProjectName := projectName
	task, err := taskBuilder.GetTaskByID(taskID)
	if err == nil {
		// 使用任务名称作为工作目录名称
		actualProjectName = task.Name
	} else {
		// 任务不存在时，使用传入的type参数来推断项目目录
		if taskType == "web" && !strings.Contains(projectName, "-web") {
			actualProjectName = projectName + "-web"
		}
	}

	// 构建完整的日志文件路径: /data/workspace/{项目名}/logs/{任务ID}/{日志文件名}
	return filepath.Join("/data/workspace", actualProjectName, "logs", taskID, logFileName)
}

// 发送当前日志
func (tc *taskLogConnection) sendCurrentLogs() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// 检查日志文件是否存在
	if _, err := os.Stat(tc.logFilePath); os.IsNotExist(err) {
		err := tc.conn.WriteMessage(websocket.TextMessage, []byte("日志文件不存在或尚未生成"))
		if err != nil {
			common.Logger.Errorf("发送消息失败: %v", err)
		}
		return
	}

	// 从指定的日志文件读取内容
	content, err := os.ReadFile(tc.logFilePath)
	if err != nil {
		common.Logger.Warnf("读取日志文件失败: %v", err)
		return
	}

	// 发送日志内容（限制行数）
	if len(content) > 0 {
		// 按行分割内容
		lines := common.SplitLines(string(content))

		// 如果行数超过限制，只取最后maxLines行
		if len(lines) > tc.maxLines {
			sendLines := lines[len(lines)-tc.maxLines:]
			// 添加提示信息
			prefixMsg := fmt.Sprintf("[日志过长，仅显示最后%d行，总共%d行]\n", tc.maxLines, len(lines))
			sendContent := prefixMsg + strings.Join(sendLines, "\n")

			err := tc.conn.WriteMessage(websocket.TextMessage, []byte(sendContent))
			if err != nil {
				common.Logger.Errorf("发送日志失败: %v", err)
				return
			}
		} else {
			// 发送全部内容
			err := tc.conn.WriteMessage(websocket.TextMessage, content)
			if err != nil {
				common.Logger.Errorf("发送日志失败: %v", err)
				return
			}
		}
		// 无论是否截取，都要正确设置文件位置为实际文件大小
		tc.lastFilePos = int64(len(content))
	}
}

// 监听任务日志更新
func (tc *taskLogConnection) watchTaskLogs() {
	ticker := time.NewTicker(500 * time.Millisecond) // 降低检查频率到500ms
	defer ticker.Stop()

	for {
		select {
		case <-tc.closeChan:
			return
		case <-ticker.C:
			// 检查日志文件是否有更新
			logPath := tc.logFilePath
			fileInfo, err := os.Stat(logPath)
			if err != nil {
				// 日志文件不存在时不输出错误，静默等待
				continue
			}

			// 如果文件大小有变化，读取新增内容
			if fileInfo.Size() > tc.lastFilePos {
				file, err := os.Open(logPath)
				if err != nil {
					common.Logger.Errorf("打开日志文件失败: %v", err)
					continue
				}

				// 从上次位置开始读取
				file.Seek(tc.lastFilePos, 0)
				buffer := make([]byte, fileInfo.Size()-tc.lastFilePos)
				n, err := file.Read(buffer)
				file.Close()

				if err != nil {
					common.Logger.Errorf("读取日志文件失败: %v", err)
					continue
				}

				if n > 0 {
					// 解析新增日志
					newContent := string(buffer[:n])
					newLogs := common.SplitLines(newContent)

					// 添加到缓冲区，而不是立即发送
					tc.mu.Lock()
					for _, log := range newLogs {
						if log == "" {
							continue
						}
						tc.logBuffer = append(tc.logBuffer, "LOG:"+log)
						tc.bufferSize++
					}
					tc.mu.Unlock()
				}

				// 更新文件位置
				tc.lastFilePos = fileInfo.Size()
			}
		}
	}
}

// 定期刷新缓冲区
func (tc *taskLogConnection) flushBufferRoutine() {
	defer tc.flushTicker.Stop()

	for {
		select {
		case <-tc.closeChan:
			return
		case <-tc.flushTicker.C:
			tc.flushBuffer()
		}
	}
}

// 刷新缓冲区，发送积累的日志
func (tc *taskLogConnection) flushBuffer() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.bufferSize == 0 {
		return
	}

	// 构建批量消息
	var buffer bytes.Buffer
	for _, log := range tc.logBuffer {
		buffer.WriteString(log + "\n")
	}

	// 发送批量消息
	err := tc.conn.WriteMessage(websocket.TextMessage, buffer.Bytes())
	if err != nil {
		common.Logger.Errorf("批量发送日志失败: %v", err)
		return
	}

	// 清空缓冲区
	tc.logBuffer = tc.logBuffer[:0]
	tc.bufferSize = 0
}

// 处理客户端消息
func (tc *taskLogConnection) handleMessages() {
	defer tc.close()

	for {
		// 读取客户端消息
		_, _, err := tc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				common.Logger.Errorf("WebSocket读取错误: %v", err)
			}
			break
		}
		// 目前我们不处理客户端发送的消息
	}
}

// 关闭连接
func (tc *taskLogConnection) close() {
	select {
	case <-tc.closeChan:
		// 已经关闭
		return
	default:
		// 在关闭前发送剩余的日志
		tc.flushBuffer()

		close(tc.closeChan)
		tc.conn.Close()
	}
}
