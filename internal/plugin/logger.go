package plugin

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Logger 为插件提供带前缀的日志输出。
type Logger struct {
	prefix string
	mu     sync.Mutex
	w      io.Writer
}

// NewLogger 创建一个新的插件日志记录器。
func NewLogger(prefix string) *Logger {
	return &Logger{
		prefix: prefix,
		w:      os.Stdout,
	}
}

func (l *Logger) log(level, format string, args ...any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := time.Now().Format("15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.w, "[%s] [%s] [%s] %s\n", ts, level, l.prefix, msg)
}

// Info 输出信息级别日志。
func (l *Logger) Info(format string, args ...any) { l.log("INFO", format, args...) }

// Warn 输出警告级别日志。
func (l *Logger) Warn(format string, args ...any) { l.log("WARN", format, args...) }

// Error 输出错误级别日志。
func (l *Logger) Error(format string, args ...any) { l.log("ERROR", format, args...) }
