package s

import (
	"strings"

	"github.com/ssgo/s/base"
)

type LogLevelType int

const LogDebug LogLevelType = 1
const LogInfo LogLevelType = 2
const LogWarning LogLevelType = 3
const LogError LogLevelType = 4

func Debug(logType string, data Map) {
	Log(LogDebug, logType, data)
}

func Info(logType string, data Map) {
	Log(LogInfo, logType, data)
}

func Warning(logType string, data Map) {
	TraceLog(LogWarning, logType, data)
}

func Error(logType string, data Map) {
	TraceLog(LogError, logType, data)
}

func Log(logLevel LogLevelType, logType string, data Map) {
	if logLevel < configedLogLevel {
		return
	}
	data["_logLevel"] = getLogLevelName(logLevel)
	base.Log(logType, data)
}

func TraceLog(logLevel LogLevelType, logType string, data Map) {
	if logLevel < configedLogLevel {
		return
	}
	data["_logLevel"] = getLogLevelName(logLevel)
	base.TraceLog(logType, data)
}

func getLogLevel(logLevel string) LogLevelType {
	switch strings.ToLower(logLevel) {
	case "debug":
		return LogDebug
	case "info":
		return LogInfo
	case "warn", "warning":
		return LogWarning
	case "error":
		return LogError
	default:
		return LogInfo
	}
}

func getLogLevelName(logLevel LogLevelType) string {
	switch logLevel {
	case LogDebug:
		return "debug"
	case LogInfo:
		return "info"
	case LogWarning:
		return "warning"
	case LogError:
		return "error"
	default:
		return "info"
	}
}
