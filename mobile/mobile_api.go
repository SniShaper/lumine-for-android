package mobile

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	lumine "github.com/moi-si/lumine/internal"

	log "github.com/moi-si/mylog"
	"github.com/xjasonlyu/tun2socks/v2/engine"
)


var (
	workingDir string
	mu         sync.Mutex
	isRunning  bool

	// Log management
	logMu      sync.Mutex
	logEntries []string
	maxLogs    = 500
	mainLogger *log.Logger
)

func init() {
	mainLogger = log.New(&LogWriter{}, "", log.LstdFlags, log.INFO)
}

func pushLog(msg string) {
	logMu.Lock()
	defer logMu.Unlock()
	logEntries = append(logEntries, msg)
	if len(logEntries) > maxLogs {
		logEntries = logEntries[1:]
	}
}

type LogWriter struct{}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	pushLog(string(p))
	return len(p), nil
}

// GetLogs 返回并清空自上次调用以来的所有日志
func GetLogs() string {
	logMu.Lock()
	defer logMu.Unlock()
	if len(logEntries) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, l := range logEntries {
		builder.WriteString(l)
	}
	logEntries = nil
	return builder.String()
}

func clearLogsLocked() {
	logEntries = nil
}



// SetWorkingDir 设置核心运行的基础路径（由 Android 端提供私有目录路径）
func SetWorkingDir(dir string) {
	mu.Lock()
	defer mu.Unlock()
	workingDir = dir
}

// StartLumine 指定配置名启动核心和 tun2socks
func StartLumine(fd int, configName string) string {
	mu.Lock()
	defer mu.Unlock()

	if workingDir == "" {
		return "working directory not set"
	}
	if isRunning {
		return ""
	}

	logMu.Lock()
	clearLogsLocked()
	logMu.Unlock()

	// Initialize logging redirection
	mainLogger.Info("Lumine mobile starting...")
	lumine.SetLogWriter(&LogWriter{})

	configPath := filepath.Join(workingDir, configName+".json")


	_, _, err := lumine.LoadConfig(configPath)
	if err != nil {
		return fmt.Sprintf("load config error: %v", err)
	}

	engine.SetCustomProxy(&LumineProxy{})

	engine.Insert(&engine.Key{
		Device:   fmt.Sprintf("fd://%d", fd),
		LogLevel: "info",
		MTU:      1500,
	})
	if err = engine.StartErr(); err != nil {
		engine.ClearCustomProxy()
		return fmt.Sprintf("engine start error: %v", err)
	}
	isRunning = true

	return ""
}

// StopLumine 停止服务
func StopLumine() {
	mu.Lock()
	defer mu.Unlock()

	if !isRunning {
		return
	}

	_ = engine.StopErr()
	engine.ClearCustomProxy()
	isRunning = false
	lumine.SetLogWriter(nil)
}

// IsRunning 返回核心当前是否正在运行。
func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return isRunning
}

// CheckConfig 验证 JSON 语法与 Lumine 规则合法性
func CheckConfig(jsonContent string) string {
	var conf lumine.Config
	if err := json.Unmarshal([]byte(jsonContent), &conf); err != nil {
		return fmt.Sprintf("invalid json: %v", err)
	}
	return ""
}

// GetVersion 返回版本号
func GetVersion() string {
	return "v0.7.9-android"
}

// HelloSplice 返回当前平台对 splice 的可用性说明。
func HelloSplice() string {
	if runtime.GOOS == "linux" {
		return "Splice is available on Linux/Android"
	}
	return "Splice is NOT available on " + runtime.GOOS
}
