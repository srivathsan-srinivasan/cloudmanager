package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	initOnce sync.Once
	mu       sync.Mutex
	logger   *log.Logger
	logFile  *os.File
	logPath  string
	initErr  error
)

func Init(version, buildTime, backend string) error {
	initOnce.Do(func() {
		logPath = defaultPath()
		logFile, initErr = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if initErr != nil {
			return
		}
		logger = log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)
		logger.Printf("level=INFO component=app event=start version=%s build=%s backend=%s pid=%d", clean(version), clean(buildTime), clean(strings.ToUpper(strings.TrimSpace(backend))), os.Getpid())
	})
	return initErr
}

func Path() string {
	if logPath != "" {
		return logPath
	}
	logPath = defaultPath()
	return logPath
}

func Infof(format string, args ...any) {
	write("INFO", format, args...)
}

func Warnf(format string, args ...any) {
	write("WARN", format, args...)
}

func Errorf(format string, args ...any) {
	write("ERROR", format, args...)
}

func ReadTail(maxBytes int) (string, error) {
	path := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("No log file found yet at %s", path), nil
		}
		return "", err
	}
	if len(data) == 0 {
		return fmt.Sprintf("Log file is empty: %s", path), nil
	}
	if maxBytes > 0 && len(data) > maxBytes {
		data = data[len(data)-maxBytes:]
		if idx := strings.IndexByte(string(data), '\n'); idx >= 0 && idx+1 < len(data) {
			data = data[idx+1:]
		}
	}
	return string(data), nil
}

func write(level, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if logger == nil {
		return
	}
	logger.Printf("level=%s %s", clean(level), clean(fmt.Sprintf(format, args...)))
}

func clean(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "\n", " | ")
}

func defaultPath() string {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".cloudmanager.log")
	}
	return filepath.Join(os.TempDir(), "cloudmanager.log")
}
