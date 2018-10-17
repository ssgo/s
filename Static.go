package s

import (
	"net/http"
	"os"
	"strings"
	"time"
)

var statics = make(map[string]*string)

func Static(path, rootPath string) {
	if rootPath[0] != '/' {
		pos := strings.LastIndexByte(os.Args[0], '/')
		if pos > 0 {
			rootPath = os.Args[0][0:pos+1] + rootPath
		}
	}
	statics[path] = &rootPath
}

func processStatic(requestPath string, request *http.Request, response *Response, headers *map[string]string, startTime *time.Time) bool {
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

	http.ServeFile(response, request, *rootPath+requestPath)

	writeLog("STATIC", nil, 0, request, response, nil, headers, startTime, 0, nil)

	return true
}
