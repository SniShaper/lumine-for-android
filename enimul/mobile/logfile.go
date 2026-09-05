package mobile

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// 会话日志落盘：与桌面端"日志文件+轮转"对齐的移动端简化实现。
// 路径 <workingDir>/logs/lumine.log；超过 maxLogFileBytes 后轮转为
// lumine.<序号>.log，最多保留 keepLogFiles 个，最旧删除。

const (
	maxLogFileBytes = 1 << 20 // 1 MiB
	keepLogFiles    = 3
)

var (
	logFileMu   sync.Mutex
	logFile     *os.File
	logFilePath string
	logFileSize int64
	logToFile   = true
)

func setLogFilesEnabled(enabled bool) {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	logToFile = enabled
	if !enabled {
		closeLogFileLocked()
	}
}

func closeLogFileLocked() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
		logFilePath = ""
		logFileSize = 0
	}
}

func ensureLogDirLocked() (string, error) {
	if workingDir == "" {
		return "", fmt.Errorf("working directory not set")
	}
	dir := filepath.Join(workingDir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func rotateLogFileLocked(dir string) error {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
	base := filepath.Join(dir, "lumine.log")
	// 移除超出保留数的最旧备份，整体后移一位。
	for i := keepLogFiles; i >= 1; i-- {
		dst := filepath.Join(dir, fmt.Sprintf("lumine.%d.log", i))
		src := filepath.Join(dir, fmt.Sprintf("lumine.%d.log", i-1))
		if i == 1 {
			src = base
		}
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if i == keepLogFiles {
			_ = os.Remove(dst)
		}
		_ = os.Rename(src, dst)
	}
	logFileSize = 0
	return openLogFileLocked(dir)
}

func openLogFileLocked(dir string) error {
	f, err := os.OpenFile(filepath.Join(dir, "lumine.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	logFile = f
	logFilePath = filepath.Join(dir, "lumine.log")
	if st, err := f.Stat(); err == nil {
		logFileSize = st.Size()
	}
	return nil
}

func writeSessionLog(b []byte) {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if !logToFile || len(b) == 0 {
		return
	}
	if logFile == nil {
		dir, err := ensureLogDirLocked()
		if err != nil {
			return
		}
		if err := openLogFileLocked(dir); err != nil {
			return
		}
	}
	if logFileSize > 0 && logFileSize+int64(len(b)) > maxLogFileBytes {
		dir, err := ensureLogDirLocked()
		if err != nil {
			return
		}
		if err := rotateLogFileLocked(dir); err != nil {
			return
		}
	}
	n, err := logFile.Write(b)
	if n > 0 {
		logFileSize += int64(n)
	}
	if err != nil {
		_ = logFile.Close()
		logFile = nil
		logFileSize = 0
	}
}

func closeSessionLog() {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	closeLogFileLocked()
}
