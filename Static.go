package s

import (
	"net/http"
	"os"
	"strings"
	"time"
)

var statics = make(map[string]*string)

func Static(path, rootPath string) {
	statics[path] = &rootPath
}

func processStatic(requestPath string, request *http.Request, response *http.ResponseWriter, headers *map[string]string, startTime *time.Time) bool {
	if len(statics) == 0 {
		return false
	}

	rootPath := statics[requestPath]
	if rootPath == nil {
		for p1, p2 := range statics {
			if strings.HasPrefix(requestPath, p1) {
				rootPath = p2
				break
			}
		}
	}

	if rootPath == nil {
		return false
	}

	filePath := *rootPath + requestPath
	if strings.HasSuffix(filePath, "/") {
		filePath += "index.html"
	}

	_, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	if strings.HasSuffix(filePath, "/index.html") {
		filePath = filePath[len(filePath)-11:]
	}

	http.ServeFile(*response, request, *rootPath+requestPath)

	writeLog("STATIC", nil, false, request, response, nil, headers, startTime, 0, 200)

	return true
}
