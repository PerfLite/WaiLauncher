package launcher

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logMu       sync.Mutex
	logFile     *os.File
	LogPath     string
	initialized bool
)

// InitLogger initializes the global log file in the specified root folder.
func InitLogger(rootDir string) error {
	logMu.Lock()
	defer logMu.Unlock()

	if initialized && logFile != nil {
		return nil
	}

	_ = os.MkdirAll(rootDir, 0755)
	LogPath = filepath.Join(rootDir, "launcher.log")

	f, err := os.OpenFile(LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	logFile = f

	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	initialized = true
	line := fmt.Sprintf("================ WaiLauncher Started [%s] ================\n", time.Now().Format("2006-01-02 15:04:05"))
	_, _ = f.WriteString(line)
	_ = f.Sync()
	return nil
}

// LogToFile appends a timestamped line directly to launcher.log and flushes it immediately.
func LogToFile(prefix, msg string) {
	logMu.Lock()
	defer logMu.Unlock()

	line := fmt.Sprintf("[%s] [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), prefix, msg)
	if logFile != nil {
		_, _ = logFile.WriteString(line)
		_ = logFile.Sync()
	}
	fmt.Print(line)
}

// LogInfo logs an informational message with immediate flush.
func LogInfo(format string, v ...interface{}) {
	LogToFile("INFO", fmt.Sprintf(format, v...))
}

// LogWarn logs a warning message with immediate flush.
func LogWarn(format string, v ...interface{}) {
	LogToFile("WARN", fmt.Sprintf(format, v...))
}

// LogError logs an error message with immediate flush.
func LogError(format string, v ...interface{}) {
	LogToFile("ERROR", fmt.Sprintf(format, v...))
}

// WailsLogger bridges Wails internal logs directly into launcher.log.
type WailsLogger struct{}

func (w *WailsLogger) Print(message string)   { LogToFile("WAILS", message) }
func (w *WailsLogger) Trace(message string)   { LogToFile("TRACE", message) }
func (w *WailsLogger) Debug(message string)   { LogToFile("DEBUG", message) }
func (w *WailsLogger) Info(message string)    { LogToFile("INFO", message) }
func (w *WailsLogger) Warning(message string) { LogToFile("WARN", message) }
func (w *WailsLogger) Error(message string)   { LogToFile("ERROR", message) }
func (w *WailsLogger) Fatal(message string)   { LogToFile("FATAL", message) }

func GetWailsLogger() *WailsLogger {
	return &WailsLogger{}
}
