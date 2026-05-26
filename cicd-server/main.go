package main

import (
	"cicd-server/common"
	"cicd-server/config"
	"cicd-server/database"
	"cicd-server/router"
	"cicd-server/taskBuilder"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 初始化配置
	cfg, err := config.InitConfig("")
	if err != nil {
		log.Fatalf("初始化配置失败: %v", err)
	}

	// 初始化日志系统
	common.InitLogger()

	// 输出日志配置信息
	common.Logger.Infof("日志配置: 启用=%v, 级别=%s", cfg.Logs.Enable, cfg.Logs.Level)

	// 初始化MySQL数据库
	dbConfig := database.DBConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Database: cfg.Database.Database,
	}
	if err := database.InitDB(dbConfig); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	common.Logger.Info("数据库初始化完成")
	defer database.CloseDB()

	// 加载项目配置（从数据库）
	if err := config.InitProjectsConfig(""); err != nil {
		log.Fatalf("加载项目配置失败: %v", err)
	}
	common.Logger.Info("项目配置加载完成")

	// 初始化路由
	mux := router.InitRouter()

	// 初始化任务管理器（单例模式，仅获取实例）
	_ = taskBuilder.GetTaskManager()
	common.Logger.Info("任务管理器初始化完成")

	// 创建HTTP服务器
	server := &http.Server{
		Addr:    ":8083", // 暂时硬编码端口，后续可以从配置读取
		Handler: mux,
	}

	// 启动HTTP服务器
	go func() {
		common.Logger.Infof("HTTP服务器正在启动，监听端口: 8083")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			common.Logger.Fatalf("HTTP服务器启动失败: %v", err)
		}
	}()

	common.Logger.Info("CICD服务器启动成功！")

	// 等待终止信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	common.Logger.Info("正在关闭CICD服务器...")

	// 优雅关闭HTTP服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		common.Logger.Errorf("HTTP服务器关闭错误: %v", err)
	}

	common.Logger.Info("CICD服务器已关闭！")
}
