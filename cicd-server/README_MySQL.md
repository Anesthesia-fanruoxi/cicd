# CICD Server - MySQL版本快速开始

## 快速部署（5分钟上手）

### 1. 启动MySQL数据库

```bash
# 进入项目目录
cd /path/to/cicd-server

# 修改密码（重要！修改docker-compose.yml中的密码）
vim docker-compose.yml
# 修改以下内容：
# MYSQL_ROOT_PASSWORD: 你的root密码
# MYSQL_PASSWORD: 你的cicd密码

# 启动MySQL
docker-compose up -d mysql

# 等待30秒让MySQL初始化完成
sleep 30

# 验证数据已导入（应该显示22）
docker-compose exec mysql mysql -ucicd -p你的cicd密码 cicd -e "SELECT COUNT(*) FROM projects;"
```

### 2. 配置cicd-server

编辑 `config/config.yml`：

```yaml
database:
  host: "127.0.0.1"
  port: 3306
  user: "cicd"
  password: "你的cicd密码"  # 与docker-compose.yml中保持一致
  database: "cicd"

cicd:
  encryption_salt: "DqJHGSTaw11yWhyjhMmiX1hgd3AoYARg"  # 与CMDB保持一致
  harbor: "https://hub.hzbxhd.com"
```

### 3. 安装依赖并启动

```bash
# 安装MySQL驱动
go get github.com/go-sql-driver/mysql

# 编译
go build -o cicd-server main.go

# 运行
./cicd-server

# 或使用systemd
systemctl restart cicd-server
```

### 4. 验证服务

```bash
# 查看日志确认数据库连接成功
tail -f logs/cicd.log

# 应该看到类似输出：
# 数据库初始化完成
# API路由注册完成
# HTTP服务器正在启动，监听端口: 8083
```

## API使用示例

### CMDB推送配置更新

```bash
# Python示例
import requests
import hashlib
import time

def update_cicd_config(name, git_url):
    timestamp = str(int(time.time()))
    salt = "DqJHGSTaw11yWhyjhMmiX1hgd3AoYARg"  # 与cicd-server配置一致
    
    # 生成签名
    sign_str = f"{name}|{git_url}|{timestamp}|{salt}"
    sign = hashlib.md5(sign_str.encode()).hexdigest()
    
    # 推送配置
    response = requests.post(
        "http://cicd-server:8083/api/config/update",
        json={
            "name": name,
            "project": {
                "git_url": git_url,
                "description": "项目描述",
                "pro_feishu_url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxx",
                "ops_feishu_url": "https://open.feishu.cn/open-apis/bot/v2/hook/yyy",
                "java_version": "java17",
                "node_version": "",
                "enable_skywalking": True
            },
            "timestamp": timestamp,
            "sign": sign
        }
    )
    print(response.json())

# 使用
update_cicd_config("jxh", "ssh://git@xxx/jxh-service.git")
```

## 数据库管理

### 查看项目配置

```bash
# 使用命令行
docker-compose exec mysql mysql -ucicd -pcicd_password cicd

# 进入MySQL后执行查询
mysql> SELECT name, description, java_version FROM projects;
mysql> SELECT * FROM projects WHERE name='jxh'\G

# 或使用Navicat等GUI工具连接
# Host: 127.0.0.1
# Port: 3306
# User: cicd
# Password: cicd_password
# Database: cicd
```

### 手动添加/修改项目

```sql
-- 新增项目
INSERT INTO projects (name, git_url, description, java_version) 
VALUES ('new-project', 'ssh://git@xxx/new.git', '新项目', 'java17');

-- 修改项目
UPDATE projects SET java_version='java21' WHERE name='jxh';

-- 删除项目
DELETE FROM projects WHERE name='old-project';
```

### 备份与恢复

```bash
# 备份
docker-compose exec mysql mysqldump -ucicd -pcicd_password cicd > backup_$(date +%Y%m%d).sql

# 恢复
docker-compose exec -T mysql mysql -ucicd -pcicd_password cicd < backup_20251011.sql
```

## 架构说明

```
┌─────────────────┐              ┌──────────────────┐
│  CMDB平台       │              │  cicd-server     │
│                 │              │                  │
│  ┌───────────┐  │   HTTP API   │  ┌────────────┐  │
│  │ MySQL DB  │  │   (加密传输) │  │  MySQL DB  │  │
│  │ (CMDB库)  │  │─────────────>│  │ (cicd库)   │  │
│  └───────────┘  │   推送配置   │  └────────────┘  │
│                 │              │                  │
└─────────────────┘              └──────────────────┘
     主数据源                          本地缓存
```

**特点：**
- ✅ 两个独立的MySQL数据库，互不干扰
- ✅ CMDB通过HTTP API推送配置（带签名验证）
- ✅ cicd-server读取本地数据库，性能高
- ✅ CMDB短暂故障不影响cicd运行

## 常用命令

```bash
# 启动/停止MySQL
docker-compose up -d mysql
docker-compose stop mysql
docker-compose restart mysql

# 查看MySQL日志
docker-compose logs -f mysql

# 查看cicd-server日志
tail -f logs/cicd.log

# 重启cicd-server
systemctl restart cicd-server

# 查询项目配置
docker-compose exec mysql mysql -ucicd -pcicd_password cicd -e "SELECT * FROM projects;"

# 进入MySQL容器
docker-compose exec mysql bash
```

## 故障排查

### 1. MySQL连接失败

```bash
# 检查MySQL是否运行
docker-compose ps

# 检查端口
netstat -tlnp | grep 3306

# 查看MySQL日志
docker-compose logs mysql

# 检查配置文件
cat config/config.yml | grep -A 5 database
```

### 2. 数据未导入

```bash
# 检查是否有数据
docker-compose exec mysql mysql -ucicd -pcicd_password cicd -e "SELECT COUNT(*) FROM projects;"

# 如果为0，手动导入
docker-compose exec -T mysql mysql -ucicd -pcicd_password cicd < sql/init.sql
```

### 3. API签名验证失败

- 检查加密盐配置是否与CMDB一致
- 检查签名算法是否正确（name|git_url|timestamp|salt）
- 检查时间戳格式是否正确

## 更多文档

- 详细部署文档：[docs/sqlite-migration.md](docs/sqlite-migration.md)
- API接口文档：查看 `docs/sqlite-migration.md` 中的API接口说明部分

## 技术栈

- **语言**: Go
- **数据库**: MySQL 8.0
- **容器**: Docker + Docker Compose
- **驱动**: github.com/go-sql-driver/mysql

## 联系方式

如有问题，请联系运维团队。
