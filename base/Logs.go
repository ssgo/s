package base

import (
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"
)

func Log(logType string, args map[string]interface{}) {
	args["_logTime"] = int(time.Now().UnixNano() / 1000000)
	args["_logType"] = logType
	data, err := json.Marshal(args)
	if err != nil {
		log.Print(map[string]interface{}{
			"_logType":   "LogError",
			"logType":    logType,
			"logContent": args,
		})
		return
	}
	log.Print(string(data))
}

func TraceLog(logType string, args map[string]interface{}) {
	TraceLogOmit(logType, args, "")
}

func TraceLogOmit(logType string, args map[string]interface{}, exclude string) {
	traces := make([]interface{}, 0)
	for i := 1; i < 20; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		if strings.Contains(file, "/go/src/") {
			continue
		}
		if exclude != "" {
			pos := strings.Index(file, exclude)
			if pos != -1 {
				file = file[pos+9:]
			}
		}
		traces = append(traces, fmt.Sprintf("%s:%d", file, line))
	}
	args["traces"] = traces
	Log(logType, args)
}
