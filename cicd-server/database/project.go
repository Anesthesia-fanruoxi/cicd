package database

import (
	"cicd-server/models"
	"database/sql"
	"fmt"
	"time"
)

// GetProjectByName 根据项目名称查询项目配置
func GetProjectByName(name string) (*models.ProjectConfig, error) {
	query := `
		SELECT name, project_name, git_vue, git_backend, update_feishu, notify_feishu,
		       description, backend_tool, frontend_tool, enable_skywalking, created_by
		FROM projects
		WHERE name = ?
	`

	var queriedName string
	var project models.ProjectConfig
	var enableSkyWalking int

	err := db.QueryRow(query, name).Scan(
		&queriedName,
		&project.ProjectName,
		&project.GitVue,
		&project.GitBackend,
		&project.UpdateFeishu,
		&project.NotifyFeishu,
		&project.Description,
		&project.BackendTool,
		&project.FrontendTool,
		&enableSkyWalking,
		&project.CreatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("项目 %s 配置不存在", name)
	}
	if err != nil {
		return nil, fmt.Errorf("查询项目配置失败: %v", err)
	}

	// 将int转换为bool
	project.EnableSkyWalking = (enableSkyWalking == 1)

	return &project, nil
}

// GetAllProjects 查询所有项目配置
func GetAllProjects() (map[string]models.ProjectConfig, error) {
	query := `
		SELECT name, project_name, git_vue, git_backend, update_feishu, notify_feishu,
		       description, backend_tool, frontend_tool, enable_skywalking, created_by
		FROM projects
		ORDER BY name
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询项目配置失败: %v", err)
	}
	defer rows.Close()

	projects := make(map[string]models.ProjectConfig)
	for rows.Next() {
		var name string
		var project models.ProjectConfig
		var enableSkyWalking int

		err := rows.Scan(
			&name,
			&project.ProjectName,
			&project.GitVue,
			&project.GitBackend,
			&project.UpdateFeishu,
			&project.NotifyFeishu,
			&project.Description,
			&project.BackendTool,
			&project.FrontendTool,
			&enableSkyWalking,
			&project.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描项目数据失败: %v", err)
		}

		// 将int转换为bool
		project.EnableSkyWalking = (enableSkyWalking == 1)
		projects[name] = project
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历项目数据失败: %v", err)
	}

	return projects, nil
}

// UpsertProject 插入或更新项目配置（存在则更新，不存在则新增）
func UpsertProject(name string, project *models.ProjectConfig) error {
	query := `
		INSERT INTO projects (
			name, project_name, git_vue, git_backend, update_feishu, notify_feishu,
			description, backend_tool, frontend_tool, enable_skywalking, created_by, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			project_name = VALUES(project_name),
			git_vue = VALUES(git_vue),
			git_backend = VALUES(git_backend),
			update_feishu = VALUES(update_feishu),
			notify_feishu = VALUES(notify_feishu),
			description = VALUES(description),
			backend_tool = VALUES(backend_tool),
			frontend_tool = VALUES(frontend_tool),
			enable_skywalking = VALUES(enable_skywalking),
			created_by = VALUES(created_by),
			updated_at = VALUES(updated_at)
	`

	// 将bool转换为int（MySQL存储为TINYINT）
	enableSkyWalking := 0
	if project.EnableSkyWalking {
		enableSkyWalking = 1
	}

	_, err := db.Exec(query,
		name,
		project.ProjectName,
		project.GitVue,
		project.GitBackend,
		project.UpdateFeishu,
		project.NotifyFeishu,
		project.Description,
		project.BackendTool,
		project.FrontendTool,
		enableSkyWalking,
		project.CreatedBy,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("更新项目配置失败: %v", err)
	}

	return nil
}

// BatchUpsertProjects 批量插入或更新项目配置
func BatchUpsertProjects(projects map[string]models.ProjectConfig) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO projects (
			name, project_name, git_vue, git_backend, update_feishu, notify_feishu,
			description, backend_tool, frontend_tool, enable_skywalking, created_by, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			project_name = VALUES(project_name),
			git_vue = VALUES(git_vue),
			git_backend = VALUES(git_backend),
			update_feishu = VALUES(update_feishu),
			notify_feishu = VALUES(notify_feishu),
			description = VALUES(description),
			backend_tool = VALUES(backend_tool),
			frontend_tool = VALUES(frontend_tool),
			enable_skywalking = VALUES(enable_skywalking),
			created_by = VALUES(created_by),
			updated_at = VALUES(updated_at)
	`)
	if err != nil {
		return fmt.Errorf("准备SQL语句失败: %v", err)
	}
	defer stmt.Close()

	for name, project := range projects {
		enableSkyWalking := 0
		if project.EnableSkyWalking {
			enableSkyWalking = 1
		}

		_, err := stmt.Exec(
			name,
			project.ProjectName,
			project.GitVue,
			project.GitBackend,
			project.UpdateFeishu,
			project.NotifyFeishu,
			project.Description,
			project.BackendTool,
			project.FrontendTool,
			enableSkyWalking,
			project.CreatedBy,
			time.Now(),
		)
		if err != nil {
			return fmt.Errorf("执行插入/更新失败 [%s]: %v", name, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %v", err)
	}

	return nil
}
