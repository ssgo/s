package base

import (
	"fmt"
	"log"
	"runtime"
	"strings"
)

func LogWithCallers(fields ...string) {
	LogWithCallersAndExtra("", fields...)
}

func LogWithCallersAndExtra(extra string, fields ...string) {
	traces := []interface{}{strings.Join(fields, "	")}
	for i := 1; i < 20; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		if strings.Contains(file, "/go/src/") {
			continue
		}
		if extra != "" {
			pos := strings.Index(file, extra)
			if pos != -1 {
				file = file[pos+9:]
			}
		}
		traces = append(traces, fmt.Sprintf("	%s:%d", file, line))
	}
	log.Print(traces...)
}
