# CICD

> 一个基于 Go 的 CI/CD 自动化构建与部署系统，采用 Server-Agent 架构。

## 架构

```
┌──────────────┐      回调/通知       ┌──────────────┐
│  cicd-server │ <──────────────────> │  cicd-agent  │
│   (中心调度)  │   WebSocket / HTTP   │   (部署节点)  │
│   :8083      │                      │   :8081      │
└──────┬───────┘                      └──────┬───────┘
       │                                     │
  ┌────▼────┐                          ┌────▼────┐
  │  MySQL   │                          │ 目标服务器 │
  └─────────┘                          └─────────┘
```

- **cicd-server** — 中心调度服务，管理项目配置、接收构建请求、编排 CI/CD 流水线，完成后回调 agent
- **cicd-agent** — 部署节点代理，接收回调执行产物下载、解压、部署、流量切换等操作

## 项目结构

```
cicd/
├── cicd-server/              # 服务端
│   ├── api/                  # HTTP API（任务、配置、WebSocket）
│   ├── common/               # 工具：加密、飞书、日志、通知
│   ├── config/               # 配置加载 + 项目配置
│   ├── database/             # MySQL 连接与操作
│   ├── models/               # 数据模型
│   ├── router/               # 路由注册
│   ├── taskBuilder/          # 任务管理器 + Java/Web 构建器
│   ├── webTaskStep/          # Web 构建步骤
│   │   ├── 4-installDependencies/
│   │   ├── 5-webPack/
│   │   └── 6-getProduct/
│   ├── javaTaskStep/         # Java 构建步骤
│   │   ├── 4-packCode/
│   │   ├── 5-getProduct/
│   │   ├── 6-createDockerFile/
│   │   ├── 7-buildImage/
│   │   └── 8-pushImage/
│   └── main.go
│
└── cicd-agent/               # 代理端
    ├── common/               # 工具：加密、飞书、IP 白名单、日志
    ├── config/               # YAML 配置加载
    ├── router/               # 路由注册（Gin）
    ├── taskCenter/           # 回调处理、更新处理、任务取消
    ├── taskStep/
    │   ├── webBuild/         # Web 部署步骤
    │   │   ├── 7-downProduct/
    │   │   ├── 8-extractProduct/
    │   │   ├── 9-backupCurrent/
    │   │   └── 10-deployNew/
    │   └── javaBuild/        # Java 部署步骤
    │       ├── 9-pullOnline/
    │       ├── 10-tagImage/
    │       ├── 11-pushLocal/
    │       └── ...
    └── main.go
```

## 构建流水线

### Web 项目

| 步骤 | 服务端（构建）            | 代理端（部署）        |
|------|--------------------------|-----------------------|
| 1    | 创建工作目录              |                       |
| 2    | 清理工作目录              |                       |
| 3    | Git 代码克隆              |                       |
| 4    | 安装 Node.js 依赖         |                       |
| 5    | Web 打包                  |                       |
| 6    | 产物打包 + 上传 CMDB      |                       |
| 7    | → 回调（含 download_url） | 下载产物              |
| 8    |                          | 解压产物              |
| 9    |                          | 备份当前版本          |
| 10   |                          | 部署新版本            |

### Java 项目

| 步骤 | 服务端（构建）            | 代理端（部署）        |
|------|--------------------------|-----------------------|
| 1-3  | 初始化 + Git 克隆         |                       |
| 4    | Maven/Gradle 打包         |                       |
| 5    | 产物获取                  |                       |
| 6    | 生成 Dockerfile           |                       |
| 7    | 构建镜像                  |                       |
| 8    | 推送镜像到 Harbor         |                       |
| 9    | → 回调                    | 拉取线上镜像          |
| 10   |                          | 打本地标签            |
| 11   |                          | 推送到本地 Harbor     |
| 12   |                          | 检查镜像              |
| 13   |                          | 部署服务              |
| 14   |                          | 检查服务              |
| 15   |                          | 流量切换（双副本）     |
| 16   |                          | 清理旧版本            |

## 快速开始

### cicd-server

```bash
cd cicd-server

# 1. 配置 MySQL

# 2. 编辑 config/config.yml
# 3. 编译运行
go build -o cicd-server main.go
./cicd-server    # 监听 :8083
```

### cicd-agent

```bash
cd cicd-agent

# 1. 编辑 config/config.yaml（部署目录、项目列表等）
# 2. 编译运行
go build -o cicd-agent main.go
./cicd-agent     # 监听 :8081
```

## 技术栈

| 组件 | 技术 |
|------|------|
| 语言 | Go 1.21+ |
| Web 框架 | net/http（server）、Gin（agent） |
| 数据库 | MySQL |
| 配置 | Viper（server）、gopkg.in/yaml（agent） |
| 日志 | go.uber.org/zap（server）、自定义（agent） |
| 容器 | Docker |
| 消息通知 | 飞书机器人 |
| 实时通信 | WebSocket |
