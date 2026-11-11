package api

import (
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
)

// taskLogConnection 任务日志 SSE 连接
type taskLogConnection struct {
	w           http.ResponseWriter
	flusher     http.Flusher
	taskID      string
	project     string
	stepType    string
	logFilePath string
	mu          sync.Mutex
	writeMu     sync.Mutex
	closeChan   chan struct{}
	closeOnce   sync.Once
	done        chan struct{}
	lastFilePos int64
	logBuffer   []string
	bufferSize  int
	maxLines    int
}

// TaskLogSSE 任务日志 SSE 处理函数
// 客户端参数：data=<加密后的 {taskId, project, stepType, type}>
func TaskLogSSE(w http.ResponseWriter, r *http.Request) {
	encryptedData := r.URL.Query().Get("data")
	if encryptedData == "" {
		http.Error(w, "缺少加密参数", http.StatusBadRequest)
		return
	}

	decryptedData, err := common.DecryptAndDecompress(encryptedData)
	if err != nil {
		http.Error(w, "解密参数失败", http.StatusBadRequest)
		return
	}

	var params struct {
		TaskID   string `json:"taskId"`
		Project  string `json:"project"`
		StepType string `json:"stepType"`
		Type     string `json:"type"`
	}
	if err := json.Unmarshal(decryptedData, &params); err != nil {
		common.Logger.Errorf("SSE解析参数失败: %v, 原始数据: %s", err, string(decryptedData))
		http.Error(w, "解析参数失败", http.StatusBadRequest)
		return
	}
	if params.TaskID == "" {
		http.Error(w, "缺少任务ID参数", http.StatusBadRequest)
		return
	}
	if params.Project == "" {
		http.Error(w, "缺少项目名称参数", http.StatusBadRequest)
		return
	}
	if params.StepType == "" {
		http.Error(w, "缺少步骤名称参数", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "不支持流式传输", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	tc := &taskLogConnection{
		w:           w,
		flusher:     flusher,
		taskID:      params.TaskID,
		project:     params.Project,
		stepType:    params.StepType,
		logFilePath: buildLogFilePath(params.Project, params.TaskID, params.StepType, params.Type),
		closeChan:   make(chan struct{}),
		done:        make(chan struct{}),
		logBuffer:   make([]string, 0, 100),
		maxLines:    1000,
	}

	tc.sendCurrentLogs()

	go tc.watchTaskLogs()
	go tc.flushBufferRoutine()
	go func() {
		<-r.Context().Done()
		tc.close()
	}()

	<-tc.done
	common.Logger.Infof("任务日志SSE连接已关闭 task=%s step=%s", params.TaskID, params.StepType)
}

// buildLogFilePath 构建日志文件路径
func buildLogFilePath(projectName, taskID, stepType, taskType string) string {
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

	actualProjectName := projectName
	task, err := taskBuilder.GetTaskByID(taskID)
	if err == nil {
		actualProjectName = task.Name
		if task.Type == "web" && !strings.Contains(actualProjectName, "-web") {
			actualProjectName = actualProjectName + "-web"
		}
	} else if taskType == "web" && !strings.Contains(projectName, "-web") {
		actualProjectName = projectName + "-web"
	}

	return filepath.Join("/data/workspace", actualProjectName, "logs", taskID, logFileName)
}

// sendCurrentLogs 发送当前已有日志
func (tc *taskLogConnection) sendCurrentLogs() {
	if _, err := os.Stat(tc.logFilePath); os.IsNotExist(err) {
		_ = tc.writeLine("日志文件不存在或尚未生成")
		return
	}

	content, err := os.ReadFile(tc.logFilePath)
	if err != nil {
		common.Logger.Warnf("读取日志文件失败: %v", err)
		return
	}

	lines := common.SplitLines(string(content))
	if len(lines) > tc.maxLines {
		lines = lines[len(lines)-tc.maxLines:]
		_ = tc.writeLine(fmt.Sprintf("[日志过长，仅显示最后%d行]", tc.maxLines))
	}

	tc.mu.Lock()
	tc.lastFilePos = int64(len(content))
	tc.mu.Unlock()

	for _, line := range lines {
		if line == "" {
			continue
		}
		if err := tc.writeLine(line); err != nil {
			return
		}
	}
}

// watchTaskLogs 监听日志文件更新
func (tc *taskLogConnection) watchTaskLogs() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-tc.closeChan:
			return
		case <-ticker.C:
			fileInfo, err := os.Stat(tc.logFilePath)
			if err != nil || fileInfo.Size() <= tc.lastFilePos {
				continue
			}

			file, err := os.Open(tc.logFilePath)
			if err != nil {
				common.Logger.Errorf("打开日志文件失败: %v", err)
				continue
			}

			_, seekErr := file.Seek(tc.lastFilePos, 0)
			if seekErr != nil {
				_ = file.Close()
				common.Logger.Errorf("定位日志文件失败: %v", seekErr)
				continue
			}

			buffer := make([]byte, fileInfo.Size()-tc.lastFilePos)
			n, readErr := file.Read(buffer)
			_ = file.Close()
			if readErr != nil {
				common.Logger.Errorf("读取日志文件失败: %v", readErr)
				continue
			}

			if n > 0 {
				newLogs := common.SplitLines(string(buffer[:n]))
				tc.mu.Lock()
				for _, logLine := range newLogs {
					if logLine == "" {
						continue
					}
					tc.logBuffer = append(tc.logBuffer, logLine)
					tc.bufferSize++
				}
				tc.mu.Unlock()
			}

			tc.mu.Lock()
			tc.lastFilePos = fileInfo.Size()
			tc.mu.Unlock()
		}
	}
}

// flushBufferRoutine 定期刷新缓冲区并发送心跳
func (tc *taskLogConnection) flushBufferRoutine() {
	flushTicker := time.NewTicker(200 * time.Millisecond)
	heartbeatTicker := time.NewTicker(20 * time.Second)
	defer flushTicker.Stop()
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-tc.closeChan:
			tc.flushBuffer()
			return
		case <-flushTicker.C:
			tc.flushBuffer()
		case <-heartbeatTicker.C:
			tc.writeHeartbeat()
		}
	}
}

// flushBuffer 发送缓冲区日志
func (tc *taskLogConnection) flushBuffer() {
	tc.mu.Lock()
	if tc.bufferSize == 0 {
		tc.mu.Unlock()
		return
	}
	lines := tc.logBuffer
	tc.logBuffer = make([]string, 0, 100)
	tc.bufferSize = 0
	tc.mu.Unlock()

	for _, line := range lines {
		if err := tc.writeLine(line); err != nil {
			return
		}
	}
}

func (tc *taskLogConnection) writeLine(line string) error {
	token, err := common.CompressAndEncrypt([]byte(line))
	if err != nil {
		return err
	}
	tc.writeMu.Lock()
	defer tc.writeMu.Unlock()
	if _, err := fmt.Fprintf(tc.w, "data: {\"data\":\"%s\"}\n\n", token); err != nil {
		return err
	}
	tc.flusher.Flush()
	return nil
}

func (tc *taskLogConnection) writeHeartbeat() {
	tc.writeMu.Lock()
	defer tc.writeMu.Unlock()
	if _, err := fmt.Fprint(tc.w, ": ping\n\n"); err != nil {
		return
	}
	tc.flusher.Flush()
}

// close 安全关闭连接
func (tc *taskLogConnection) close() {
	tc.closeOnce.Do(func() {
		tc.flushBuffer()
		close(tc.closeChan)
		close(tc.done)
	})
}
