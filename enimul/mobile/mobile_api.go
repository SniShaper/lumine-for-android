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

func getWorkingDir() string {
	workingDirMu.RLock()
	defer workingDirMu.RUnlock()
	return workingDir
}

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

func StopLumine() {
	mu.Lock()
	defer mu.Unlock()

	if !isRunning {
		return
	}

	_ = engine.StopErr()
	engine.ClearCustomProxy()
	lumine.StopIPPools()
	dial.StopLocalAddrMonitor()
	isRunning = false
	lumine.SetLogWriter(nil)
	closeSessionLog()
}

func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return isRunning
}

func CheckConfig(jsonContent string) string {
	var conf lumine.Config
	if err := json.Unmarshal([]byte(jsonContent), &conf); err != nil {
		return fmt.Sprintf("invalid json: %v", err)
	}
	return ""
}

func GetVersion() string {
	return "enimul-" + lumine.Version + "-android"
}

func HelloSplice() string {
	if runtime.GOOS == "linux" {
		return "Splice is available on Linux/Android"
	}
	return "Splice is NOT available on " + runtime.GOOS
}

func GetStats() string {
	return lumine.SnapshotStats().JSON()
}

func OnNetworkChanged() string {
	if err := lumine.ResetRuntimeState(); err != nil {
		mainLogger.Error("network changed: reset runtime state: ", err)
	} else {
		mainLogger.Info("network changed: runtime caches cleared")
	}
	return ""
}

func SetLogFileEnabled(enabled bool) {
	setLogFilesEnabled(enabled)
}

func LogFilePath() string {
	dir := getWorkingDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "logs")
}
