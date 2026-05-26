# 日志系统说明

## 概述

CICD-Agent 实现了双层日志系统：
1. **控制台日志**：简化输出，仅显示关键信息
2. **任务日志文件**：详细记录，按任务ID和步骤分类

## 目录结构

```
logs/
└── {taskId}/
    ├── console.log         # 任务总体日志
    ├── pullOnline.log      # 步骤9: 拉取在线镜像
    ├── tagImages.log       # 步骤10: 标记镜像
    ├── pushLocal.log       # 步骤11: 推送本地镜像
    ├── checkImage.log      # 步骤12: 检查镜像
    ├── deployService.log   # 步骤13: 部署服务
    ├── checkService.log    # 步骤14: 检查服务
    ├── trafficSwitching.log # 步骤15: 流量切换
    ├── cleanupOldVersion.log # 步骤16: 清理旧版本
    ├── downProduct.log     # Web步骤7: 下载产物
    ├── extractProduct.log  # Web步骤8: 解压产物
    ├── backupCurrent.log   # Web步骤9: 备份当前版本
    └── deployNew.log       # Web步骤10: 部署新版本
```

## 控制台日志

### 输出级别
- **INFO**: 服务启动、请求接收、任务状态变更
- **ERROR**: 错误信息

### 示例
```
2025/10/09 11:30:00 [INFO] 启动CICD代理服务 地址: 0.0.0.0:8080
2025/10/09 11:30:15 [INFO] 收到更新请求: 项目=ysh-api
2025/10/09 11:30:16 [INFO] 收到构建成功回调: 项目=ysh-api, 任务ID=ysh-api-v1.0.0-1728450616
2025/10/09 11:30:16 [INFO] 开始单版本部署: 项目=ysh-api, 标签=v1.0.0, 任务ID=ysh-api-v1.0.0-1728450616
2025/10/09 11:30:20 [INFO] 步骤9: 拉取在线镜像
2025/10/09 11:30:25 [INFO] 步骤9完成
2025/10/09 11:31:50 [INFO] 单版本部署完成: 项目=ysh-api, 任务ID=ysh-api-v1.0.0-1728450616
```

## 任务日志文件

### 日志级别
- **INFO**: 普通信息
- **DEBUG**: 调试信息（仅文件）
- **WARNING**: 警告信息
- **ERROR**: 错误信息
- **COMMAND**: 命令执行记录

### console.log 示例
```
2025/10/09 11:30:16 [INFO] 开始处理单版本部署请求: 项目=ysh-api, 标签=v1.0.0, 分类=test
```

### pullOnline.log 示例
```
2025/10/09 11:30:16 [INFO] 开始拉取在线镜像
2025/10/09 11:30:16 [INFO] 镜像列表: [harbor.example.com/ysh/ysh-api:v1.0.0]
2025/10/09 11:30:16 [INFO] 拉取镜像: 总数=1, 并发数=1
2025/10/09 11:30:16 [INFO] 开始拉取镜像: harbor.example.com/ysh/ysh-api:v1.0.0
2025/10/09 11:30:16 [COMMAND] docker pull harbor.example.com/ysh/ysh-api:v1.0.0
v1.0.0: Pulling from ysh/ysh-api
e7c96db7181b: Already exists
f910a506b6cb: Already exists
...
Status: Downloaded newer image for harbor.example.com/ysh/ysh-api:v1.0.0
2025/10/09 11:30:20 [INFO] 成功拉取镜像: harbor.example.com/ysh/ysh-api:v1.0.0
2025/10/09 11:30:20 [INFO] 所有镜像拉取完成: 1个
2025/10/09 11:30:20 [INFO] 拉取在线镜像完成
```

## 日志管理

### 日志清理
- **保留时间**: 默认7天
- **清理时间**: 启动时 + 每天凌晨2点
- **配置位置**: `main.go` 中 `common.StartLogCleanupRoutine(7)`

### 修改保留天数
```go
// main.go
common.StartLogCleanupRoutine(14) // 修改为保留14天
```

## 日志查看

### 查看特定任务日志
```bash
# 查看任务总体日志
cat logs/{taskId}/console.log

# 查看特定步骤日志
cat logs/{taskId}/pullOnline.log

# 实时跟踪日志
tail -f logs/{taskId}/pullOnline.log
```

### 查找错误
```bash
# 查找任务中的错误
grep ERROR logs/{taskId}/*.log

# 查找命令执行失败
grep -A 5 "COMMAND.*failed" logs/{taskId}/*.log
```

## 已集成日志的步骤

### Java构建步骤 (步骤9-16)
- ✅ **步骤9 (pullOnline)**: 拉取在线镜像 - 完整日志记录（docker pull命令输出）
- ✅ **步骤10 (tagImages)**: 标记镜像 - 完整日志记录（docker tag命令输出）
- ✅ **步骤11 (pushLocal)**: 推送本地镜像 - 完整日志记录（docker push命令输出）
- ✅ **步骤12 (checkImage)**: 检查镜像 - 完整日志记录（Harbor API检查结果）
- ✅ **步骤13 (deployService)**: 部署服务 - 完整日志记录（kubectl apply命令输出）
- ✅ **步骤14 (checkService)**: 检查服务 - 完整日志记录（pod状态检查）
- ✅ **步骤15 (trafficSwitching)**: 流量切换 - 完整日志记录（nginx配置更新、kubectl命令）
- ✅ **步骤16 (cleanupOldVersion)**: 清理旧版本 - 完整日志记录（kubectl delete命令输出）

### Web构建步骤 (步骤7-10)
- ✅ **步骤7 (downProduct)**: 下载产物 - 完整日志记录
- ✅ **步骤8 (extractProduct)**: 解压产物 - 完整日志记录
- ✅ **步骤9 (backupCurrent)**: 备份当前版本 - 完整日志记录
- ✅ **步骤10 (deployNew)**: 部署新版本 - 完整日志记录

## 开发者指南

### 在新步骤中使用日志

```go
// 1. 在模块struct中添加taskLogger字段
type ImagePusher struct {
    taskID     string
    taskLogger *common.TaskLogger
}

// 2. 在构造函数中接收taskLogger
func NewImagePusher(taskID string, taskLogger *common.TaskLogger) *ImagePusher {
    return &ImagePusher{
        taskID:     taskID,
        taskLogger: taskLogger,
    }
}

// 3. 在关键位置记录日志
func (p *ImagePusher) PushImages(ctx context.Context, images []string) error {
    // 记录步骤开始
    if p.taskLogger != nil {
        p.taskLogger.WriteStep("pushLocal", "INFO", "开始推送镜像")
    }
    
    // 记录命令执行
    cmd := exec.CommandContext(ctx, "docker", "push", image)
    output, err := cmd.CombinedOutput()
    if p.taskLogger != nil {
        p.taskLogger.WriteCommand("pushLocal", "docker push "+image, output, err)
    }
    
    // 记录步骤完成
    if p.taskLogger != nil {
        p.taskLogger.WriteStep("pushLocal", "INFO", "镜像推送完成")
    }
    
    return nil
}

// 4. 在Processor中传递taskLogger
func (r *SingleVersionProcessor) step11PushLocal() error {
    pusher := pushLocal.NewImagePusher(r.taskID, r.taskLogger)
    return pusher.PushImages(r.ctx, images)
}
```

### 控制台日志最佳实践

1. **仅记录关键信息**：任务开始/完成、步骤状态、错误
2. **使用简洁格式**：`项目=%s, 任务ID=%s`
3. **避免详细输出**：参数、命令输出等写入文件

## 性能优化

### 当前实现
- 并发写入支持：使用 `sync.RWMutex` 保护
- 文件自动管理：创建时打开，任务结束时关闭
- 减少控制台IO：提升整体性能

### 注意事项
- 大量并发任务时，注意文件句柄数量
- 长时间运行任务，日志文件可能较大
- 定期清理旧日志，避免磁盘占满

## 故障排查

### 日志文件未创建
1. 检查 `logs/` 目录权限
2. 确认 `TaskLogger` 创建成功
3. 查看控制台错误日志

### 日志内容不完整
1. 确认任务正常结束（调用了 `defer taskLogger.Close()`）
2. 检查是否有写入错误日志
3. 验证磁盘空间充足

### 控制台日志过多
1. 检查日志级别设置
2. 确认使用 `common.AppLogger.Debug()` 而非 `Info()`
3. 移除不必要的日志输出
