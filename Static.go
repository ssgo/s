package s

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/ssgo/log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

var statics = make(map[string]*string)
var staticsByHost = make(map[string]map[string]*string)
var staticsFiles = make(map[string][]byte)
var staticsFileByHost = make(map[string]map[string][]byte)

func resetStaticMemory() {
	statics = make(map[string]*string)
	staticsByHost = make(map[string]map[string]*string)
	staticsFiles = make(map[string][]byte)
	staticsFileByHost = make(map[string]map[string][]byte)
}

func SetStaticGZFile(path string, data []byte) {
	SetStaticGZFileByHost(path, data, "")
}

func SetStaticFile(path string, data []byte) {
	SetStaticFileByHost(path, data, "")
}

func SetStaticGZFileByHost(path string, data []byte, host string) {
	if host == "" {
		staticsFiles[path] = data
	} else {
		if staticsFileByHost[host] == nil {
			staticsFileByHost[host] = make(map[string][]byte)
		}
		staticsFileByHost[host][path] = data
	}
}

func SetStaticFileByHost(path string, data []byte, host string) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(data)
	_ = gz.Close()
	SetStaticGZFileByHost(path, buf.Bytes(), host)
}

func Static(path, rootPath string) {
	StaticByHost(path, rootPath, "")
}

func StaticByHost(path, rootPath, host string) {
	rootPath = strings.ReplaceAll(rootPath, "\\", "/")
	if rootPath[0] != '/' {
		pos := strings.LastIndexByte(os.Args[0], '/')
		if pos > 0 {
			rootPath = os.Args[0][0:pos+1] + rootPath
			if rootPath[0] != '/' {
				wd, err := os.Getwd()
				if err == nil {
					rootPath = fmt.Sprintf("%s/%s", wd, rootPath)
				}
			}
		}
	}
	rootPath = margePath(rootPath)
	if host == "" {
		statics[path] = &rootPath
	} else {
		if staticsByHost[host] == nil {
			staticsByHost[host] = make(map[string]*string)
		}
		staticsByHost[host][path] = &rootPath
	}
}

func margePath(path string) string {
	for strings.Index(path, "//") != -1 {
		path = strings.ReplaceAll(path, "//", "/")
	}
	for strings.Index(path, "/./") != -1 {
		path = strings.ReplaceAll(path, "/./", "/")
	}
	for strings.Index(path, "/../") != -1 {
		posddg := strings.Index(path, "../")
		path1 := path[0 : posddg-1]
		path2 := path[posddg+3:]

		pos1 := strings.LastIndexByte(path1, '/')
		if pos1 > 0 {
			path1 = path1[0:pos1]
		}
		path = fmt.Sprintf("%s/%s", path1, path2)
	}
	return path
}

func processStatic(requestPath string, request *http.Request, response *Response, startTime *time.Time, requestLogger *log.Logger) bool {
	baseHost := strings.SplitN(request.Host, ":", 2)[0]

	// 从内存中查找
	var fileBuf []byte
	if staticsFileByHost[request.Host] != nil && staticsFileByHost[request.Host][requestPath] != nil {
		fileBuf = staticsFileByHost[request.Host][requestPath]
	} else if baseHost != request.Host && staticsFileByHost[baseHost] != nil && staticsFileByHost[baseHost][requestPath] != nil {
		fileBuf = staticsFileByHost[baseHost][requestPath]
	} else if staticsFiles[requestPath] != nil {
		fileBuf = staticsFiles[requestPath]
	}

	outLen := 0
	if fileBuf != nil {
		response.Header().Set("Content-Encoding", "gzip")
		http.ServeContent(response, request, path.Base(requestPath), serverStartTime, bytes.NewReader(fileBuf))
		outLen = len(fileBuf)
	} else {
		if len(statics) == 0 && len(staticsByHost) == 0 {
			return false
		}

		var rootPath *string
		if staticsByHost[request.Host] != nil {
			// 从虚拟主机设置中匹配
			rootPath = staticsByHost[request.Host][requestPath]
			if rootPath == nil && strings.ContainsRune(request.Host, ':') {
				fixedHost := strings.SplitN(request.Host, ":", 2)[1]
				if staticsByHost[fixedHost] != nil {
					rootPath = staticsByHost[fixedHost][requestPath]
				}
			}
		}
		// 去掉端口匹配
		if rootPath == nil && baseHost != request.Host {
			if staticsByHost[baseHost] != nil {
				// 从虚拟主机设置中匹配
				rootPath = staticsByHost[baseHost][requestPath]
				if rootPath == nil && strings.ContainsRune(baseHost, ':') {
					fixedHost := strings.SplitN(baseHost, ":", 2)[1]
					if staticsByHost[fixedHost] != nil {
						rootPath = staticsByHost[fixedHost][requestPath]
					}
				}
			}
		}

		if rootPath == nil {
			// 从全局设置中匹配
			rootPath = statics[requestPath]
			if rootPath == nil {
				for p1, p2 := range statics {
					if strings.HasPrefix(requestPath, p1) {
						rootPath = p2
						requestPath = requestPath[len(p1):]
						break
					}
				}
			} else {
				requestPath = requestPath[len(requestPath):]
			}
		}

		if rootPath == nil {
			return false
		}

		filePath := *rootPath + requestPath
		if strings.HasSuffix(filePath, "/") {
			filePath += "index.html"
		}

		//fileInfo, err := os.Stat(filePath)
		//if err != nil {
		//	return false
		//}
		//
		////if strings.HasSuffix(filePath, "/index.html") {
		////	filePath = filePath[len(filePath)-11:]
		////}
		//
		////http.ServeFile(response, request, *rootPath+requestPath)
		//
		//if Config.Compress && int(fileInfo.Size()) >= Config.CompressMinSize && int(fileInfo.Size()) <= Config.CompressMaxSize && strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") {
		//	zipWriter := NewGzipResponseWriter(response)
		//	http.ServeFile(zipWriter, request, *rootPath+requestPath)
		//	zipWriter.Close()
		//} else {
		//	http.ServeFile(response, request, *rootPath+requestPath)
		//}

		//writeLog(requestLogger, "STATIC", nil, int(fileInfo.Size()), request, response, nil, headers, startTime, 0, nil)

		size, err := ResponseStatic(filePath, request, response)
		if err != nil {
			return false
		}
		outLen = size
	}

	writeLog(requestLogger, "STATIC", nil, outLen, request, response, nil, startTime, 0, nil)
	return true
}

func ResponseStatic(filePath string, request *http.Request, response *Response) (int, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}

	if Config.Compress && int(fileInfo.Size()) >= Config.CompressMinSize && int(fileInfo.Size()) <= Config.CompressMaxSize && strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") {
		zipWriter := NewGzipResponseWriter(response)
		http.ServeFile(zipWriter, request, filePath)
		zipWriter.Close()
	} else {
		http.ServeFile(response, request, filePath)
	}

	return int(fileInfo.Size()), nil
}
