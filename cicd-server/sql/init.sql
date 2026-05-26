-- ================================================
-- CICD项目配置数据库初始化脚本（MySQL版本）
-- ================================================

-- 删除已存在的表（如果需要重新初始化）
DROP TABLE IF EXISTS projects;

-- 创建项目配置表（对齐CMDB的sys_project_dict表结构）
CREATE TABLE projects (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    name VARCHAR(64) NOT NULL COMMENT '项目键（唯一标识）',
    project_name VARCHAR(255) NOT NULL COMMENT '项目中文名',
    git_vue VARCHAR(255) DEFAULT '' COMMENT '前端git仓库',
    git_backend VARCHAR(255) DEFAULT '' COMMENT '后端git仓库',
    update_feishu VARCHAR(255) DEFAULT '' COMMENT '发版通知地址（替代ops飞书）',
    notify_feishu VARCHAR(255) DEFAULT '' COMMENT '普通通知地址（替代pro飞书）',
    description VARCHAR(500) DEFAULT '' COMMENT '项目描述',
    backend_tool VARCHAR(255) DEFAULT '' COMMENT '后端工具（java17等）',
    frontend_tool VARCHAR(255) DEFAULT '' COMMENT '前端工具（node14等）',
    enable_skywalking BOOLEAN DEFAULT false COMMENT '是否启用skywalking',
    created_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE INDEX uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目配置表';

