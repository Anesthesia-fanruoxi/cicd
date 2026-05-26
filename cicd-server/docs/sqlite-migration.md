# 项目配置MySQL部署指南

## 概述

项目配置管理已从JSON文件迁移到MySQL数据库，支持CMDB推送更新配置。

## 架构变更

### 之前（JSON文件）
- 配置存储：`config/projects.json`
- 读取方式：直接读取文件
- 更新方式：手动修改文件
- 问题：并发不安全、无法远程更新

### 现在（MySQL数据库）
- 配置存储：独立的MySQL数据库
- 读取方式：从MySQL查询（带内存缓存）
- 更新方式：CMDB通过API推送更新
- 优势：并发安全、支持远程更新、事务保证、专业运维工具支持

## 部署步骤

### 1. 使用Docker Compose启动MySQL（推荐）

```bash
# 进入项目目录
cd /path/to/cicd-server

# 修改docker-compose.yml中的密码（重要！）
# 修改 MYSQL_ROOT_PASSWORD 和 MYSQL_PASSWORD

# 启动MySQL容器
docker-compose up -d mysql

# 查看容器状态
docker-compose ps

# 查看日志确认启动成功
docker-compose logs -f mysql

# 等待MySQL初始化完成（首次启动会自动执行init.sql）
# 验证数据
docker-compose exec mysql mysql -ucicd -pcicd_password cicd -e "SELECT COUNT(*) FROM projects;"
# 应该显示：22（当前项目数量）
```

### 2. 或手动安装MySQL

```bash
# CentOS/RHEL
sudo yum install mysql-server
sudo systemctl start mysqld
sudo systemctl enable mysqld

# 创建数据库和用户
mysql -uroot -p << EOF
CREATE DATABASE cicd DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'cicd'@'localhost' IDENTIFIED BY 'cicd_password';
GRANT ALL PRIVILEGES ON cicd.* TO 'cicd'@'localhost';
FLUSH PRIVILEGES;
EOF

# 导入初始化SQL
mysql -ucicd -pcicd_password cicd < sql/init.sql

# 验证数据
mysql -ucicd -pcicd_password cicd -e "SELECT COUNT(*) FROM projects;"
```

### 3. 配置加密盐（重要）

编辑 `config/config.yml`：

```yaml
cicd:
  encryption_salt: "your-secret-salt-here"  # 修改为你的加密盐
  harbor: "hub.hzbxhd.com"
```

**注意**：加密盐用于验证CMDB推送的签名，必须与CMDB配置一致。

### 4. 配置数据库连接

编辑 `config/config.yml`：

```yaml
database:
  host: "127.0.0.1"        # 使用Docker Compose则填 "127.0.0.1"
  port: 3306
  user: "cicd"
  password: "cicd_password"  # 修改为你设置的密码
  database: "cicd"
```

### 5. 安装依赖

```bash
go get github.com/go-sql-driver/mysql
```

### 6. 编译部署

```bash
# 编译
go build -o cicd-server main.go

# 重启服务
systemctl restart cicd-server
```

## API接口说明

### 1. 更新单个项目配置

**接口**：`POST /api/config/update`

**请求示例**：
```bash
curl -X POST http://localhost:8083/api/config/update \
  -H "Content-Type: application/json" \
  -d '{
    "name": "jxh",
    "project": {
      "git_url": "ssh://git@39.103.152.93:9922/jie-xiang-hua/jxh-service.git",
      "description": "戒享花",
      "pro_feishu_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxx",
      "ops_feishu_url": "https://open.feishu.cn/open-apis/bot/v2/hook/yyy",
      "java_version": "java17",
      "node_version": "",
      "enable_skywalking": true
    },
    "timestamp": "1635739200",
    "sign": "计算的MD5签名"
  }'
```

**签名算法**：
```
signStr = name + "|" + git_url + "|" + timestamp + "|" + salt
sign = md5(signStr)
```

**响应**：
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "message": "项目 jxh 配置更新成功"
  }
}
```

### 2. 批量更新项目配置

**接口**：`POST /api/config/batch-update`

**请求示例**：
```bash
curl -X POST http://localhost:8083/api/config/batch-update \
  -H "Content-Type: application/json" \
  -d '{
    "projects": {
      "jxh": { ... },
      "axh": { ... }
    },
    "timestamp": "1635739200",
    "sign": "计算的MD5签名"
  }'
```

**签名算法**：
```
names = sorted(project_names)  # 按字母排序
signStr = name1 + "|" + name2 + "|" + ... + "|" + timestamp + "|" + salt
sign = md5(signStr)
```

## CMDB集成说明

### CMDB需要实现的功能

1. **配置变更时推送**：
   - 用户在CMDB修改项目配置时
   - CMDB保存到自己的数据库
   - 调用cicd-server的更新接口推送配置

2. **签名生成**：
   ```python
   import hashlib
   
   def generate_sign(name, git_url, timestamp, salt):
       sign_str = f"{name}|{git_url}|{timestamp}|{salt}"
       return hashlib.md5(sign_str.encode()).hexdigest()
   ```

3. **推送代码示例**：
   ```python
   import requests
   import hashlib
   import time
   
   def sync_project_to_cicd(name, project_config):
       timestamp = str(int(time.time()))
       salt = "your-secret-salt"  # 与cicd-server配置一致
       
       # 生成签名
       sign_str = f"{name}|{project_config['git_url']}|{timestamp}|{salt}"
       sign = hashlib.md5(sign_str.encode()).hexdigest()
       
       # 推送到cicd-server
       response = requests.post(
           "http://cicd-server:8083/api/config/update",
           json={
               "name": name,
               "project": project_config,
               "timestamp": timestamp,
               "sign": sign
           }
       )
       return response.json()
   ```

## 数据备份

### 备份数据库

**使用Docker Compose：**
```bash
# 备份到SQL文件
docker-compose exec mysql mysqldump -ucicd -pcicd_password cicd > backup_$(date +%Y%m%d).sql

# 定时备份（添加到crontab）
0 2 * * * cd /path/to/cicd-server && docker-compose exec mysql mysqldump -ucicd -pcicd_password cicd > /data/backup/cicd_$(date +\%Y\%m\%d).sql
```

**手动安装的MySQL：**
```bash
# 备份到SQL文件
mysqldump -ucicd -pcicd_password cicd > backup_$(date +%Y%m%d).sql

# 定时备份（添加到crontab）
0 2 * * * mysqldump -ucicd -pcicd_password cicd > /data/backup/cicd_$(date +\%Y\%m\%d).sql
```

### 恢复数据库

**使用Docker Compose：**
```bash
# 从备份恢复
docker-compose exec -T mysql mysql -ucicd -pcicd_password cicd < backup_20251011.sql
```

**手动安装的MySQL：**
```bash
# 从备份恢复
mysql -ucicd -pcicd_password cicd < backup_20251011.sql
```

## 常见问题

### 1. 签名验证失败
- 检查加密盐配置是否一致
- 检查时间戳是否正确
- 检查签名算法是否一致

### 2. 数据库连接失败
```bash
# 检查MySQL是否启动
docker-compose ps
# 或
systemctl status mysqld

# 检查端口是否监听
netstat -tlnp | grep 3306

# 查看MySQL日志
docker-compose logs mysql
# 或
tail -f /var/log/mysqld.log
```

### 3. 查询数据库
```bash
# Docker Compose方式
docker-compose exec mysql mysql -ucicd -pcicd_password cicd -e "SELECT name, java_version FROM projects;"

# 手动安装方式
mysql -ucicd -pcicd_password cicd -e "SELECT name, java_version FROM projects;"

# 查看特定项目
mysql -ucicd -pcicd_password cicd -e "SELECT * FROM projects WHERE name='jxh';"

# 使用Navicat等GUI工具连接
# Host: 127.0.0.1
# Port: 3306
# User: cicd
# Password: cicd_password
# Database: cicd
```

### 4. Docker容器管理
```bash
# 启动MySQL
docker-compose up -d mysql

# 停止MySQL
docker-compose stop mysql

# 重启MySQL
docker-compose restart mysql

# 查看日志
docker-compose logs -f mysql

# 进入MySQL容器
docker-compose exec mysql bash

# 删除容器和数据（危险操作！）
docker-compose down -v
```

## 兼容性说明

- ✅ 现有代码完全兼容，`config.GetProjectConfig()` 接口保持不变
- ✅ 支持内存缓存，查询性能不受影响
- ✅ 支持存在则更新、不存在则新增的upsert操作
- ⚠️ `special_lists` 暂未迁移到数据库，如需使用请单独处理
