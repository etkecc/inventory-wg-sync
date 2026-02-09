package utils

import "log"

var (
	logger    *log.Logger
	withDebug bool
)

// SetLogger configures the shared logger for the app.
func SetLogger(l *log.Logger) {
	logger = l
}

// SetDebug enables or disables debug logging.
func SetDebug(enabled bool) {
	withDebug = enabled
}

// Log writes to the shared logger if configured.
func Log(args ...any) {
	if logger == nil {
		return
	}
	logger.Println(args...)
}

// Debug writes to the shared logger only when debug logging is enabled.
func Debug(args ...any) {
	if !withDebug {
		return
	}
	Log(args...)
}
