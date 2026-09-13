package mobile

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	lumine "github.com/lzpls/enimul/internal/core"
	"github.com/lzpls/enimul/internal/dial"
	"github.com/lzpls/enimul/internal/log"

	"github.com/xjasonlyu/tun2socks/v2/engine"
)

var (
	workingDir   string
	workingDirMu sync.RWMutex
	mu           sync.Mutex
	isRunning    bool

	// Log management
	logMu      sync.Mutex
	logEntries []string
	maxLogs    = 500
	mainLogger log.Logger
)

func init() {
	mainLogger = lumine.NewSessionLogger("")
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
	writeSessionLog(p)
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

// getWorkingDir 返回当前工作目录（锁保护读取；workingDirMu 为叶子锁，
// 不会与 mu/logFileMu 形成嵌套，避免锁序反转）。
func getWorkingDir() string {
	workingDirMu.RLock()
	defer workingDirMu.RUnlock()
	return workingDir
}

// SetWorkingDir 设置核心运行的基础路径（由 Android 端提供私有目录路径）
func SetWorkingDir(dir string) {
	workingDirMu.Lock()
	defer workingDirMu.Unlock()
	workingDir = dir
}

// safeConfigNameChars is the allowlist for configName characters (E10).
// Only [A-Za-z0-9_-] is permitted to prevent path traversal via filepath.Join.
const safeConfigNameChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"

// isSafeConfigName returns true if name contains only characters from safeConfigNameChars
// and is non-empty.
func isSafeConfigName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, r := range name {
		if !strings.ContainsRune(safeConfigNameChars, r) {
			return false
		}
	}
	return true
}

// StartLumine 指定配置名启动核心和 tun2socks
func StartLumine(fd int, configName string) string {
	mu.Lock()
	defer mu.Unlock()

	if getWorkingDir() == "" {
		return "working directory not set"
	}
	if isRunning {
		return ""
	}

	// E10: validate configName to prevent path traversal — only [A-Za-z0-9_-] allowed
	if !isSafeConfigName(configName) {
		return fmt.Sprintf("invalid config name: %q (only [A-Za-z0-9_-] allowed)", configName)
	}

	logMu.Lock()
	clearLogsLocked()
	logMu.Unlock()

	lumine.ResetStats()

	configPath := filepath.Join(getWorkingDir(), configName+".json")

	_, _, err := lumine.LoadConfig(configPath)
	if err != nil {
		return fmt.Sprintf("load config error: %v", err)
	}

	// Initialize logging redirection
	mainLogger.Info("Lumine mobile starting...")
	lumine.SetLogWriter(&LogWriter{})

	engine.SetCustomProxy(&LumineProxy{})

	// The engine's fdbased device takes the raw fd and closes it with a raw
	// close(2), which bypasses Android's fdsan ownership bookkeeping. Passing
	// the VpnService fd directly would leave a stale ownership tag that trips
	// fdsan later when the fd number is reused. Duplicate the fd first so the
	// engine owns an untagged copy; the original is closed by the Java side.
	tunFd, err := dupFd(fd)
	if err != nil {
		return fmt.Sprintf("dup tun fd error: %v", err)
	}
	mainLogger.Info(fmt.Sprintf("tun fd provenance: original=%d engine_dup=%d (untagged, closed by engine on stop)", fd, tunFd))

	engine.Insert(&engine.Key{
		Device:   fmt.Sprintf("fd://%d", tunFd),
		LogLevel: "info",
		MTU:      1500,
	})
	if err = engine.StartErr(); err != nil {
		engine.ClearCustomProxy()
		_ = closeFd(tunFd)
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
	// 停止 IP 池扫描与本地地址监测的周期 goroutine，避免跨会话累积。
	lumine.StopIPPools()
	dial.StopLocalAddrMonitor()
	isRunning = false
	lumine.SetLogWriter(nil)
	closeSessionLog()
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
	return "enimul-" + lumine.Version + "-android"
}

// HelloSplice 返回当前平台对 splice 的可用性说明。
func HelloSplice() string {
	if runtime.GOOS == "linux" {
		return "Splice is available on Linux/Android"
	}
	return "Splice is NOT available on " + runtime.GOOS
}

// GetStats 返回会话统计 JSON：{down,up,blocked,tcp_conns,udp_conns,uptime_ms}。
func GetStats() string {
	return lumine.SnapshotStats().JSON()
}

// OnNetworkChanged 在底层默认网络切换（Wi-Fi<->蜂窝、onLost/onAvailable）后调用，
// 清空 DNS/反向解析/TTL 缓存使后续解析走当前网络路径。
func OnNetworkChanged() string {
	if err := lumine.ResetRuntimeState(); err != nil {
		mainLogger.Error("network changed: reset runtime state: ", err)
	} else {
		mainLogger.Info("network changed: runtime caches cleared")
	}
	return ""
}

// SetLogFileEnabled 开关会话日志落盘（<workingDir>/logs/lumine*.log），默认开启。
func SetLogFileEnabled(enabled bool) {
	setLogFilesEnabled(enabled)
}

// LogFilePath 返回当前会话日志目录路径，供宿主应用展示/清理（未设置工作目录时返回空）。
func LogFilePath() string {
	dir := getWorkingDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "logs")
}
