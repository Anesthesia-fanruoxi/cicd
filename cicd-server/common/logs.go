package common

import (
	"cicd-server/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
	"strings"
	"time"
)

var Logger *zap.SugaredLogger
var StartupLogger *zap.SugaredLogger // 新增启动日志记录器

// 根据配置获取日志级别
func getLogLevel(levelStr string) zapcore.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic":
		return zapcore.DPanicLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel // 默认为Info级别
	}
}

// InitLogger 日志配置
func InitLogger() {
	// 设置日志输出格式
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     timeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 从配置读取日志开关状态和日志级别
	cfg := config.GetConfig()
	enableLogs := cfg.Logs.Enable
	logLevel := getLogLevel(cfg.Logs.Level)

	// 初始化启动日志记录器 - 这个始终输出到控制台
	startupCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)
	startupLoggerObj := zap.New(startupCore, zap.AddCaller())
	StartupLogger = startupLoggerObj.Sugar()

	// 根据配置决定常规日志输出
	var writer zapcore.WriteSyncer
	if enableLogs {
		// 启用日志时使用标准输出
		writer = zapcore.AddSync(os.Stdout)
	} else {
		// 禁用日志时使用空写入器
		writer = zapcore.AddSync(io.Discard)
	}

	// 创建日志核心
	stdCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		writer,
		logLevel, // 使用配置的日志级别
	)

	// 初始化日志器
	logger := zap.New(stdCore, zap.AddCaller())
	Logger = logger.Sugar()

	// 输出日志系统状态 - 使用StartupLogger确保始终可见
	StartupLogger.Infof("日志系统初始化完成，日志输出状态: %v，日志级别: %s", enableLogs, cfg.Logs.Level)
}

// 自定义时间格式
func timeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}
