package s

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ssgo/log"
	"github.com/ssgo/u"
)

var statics = make(map[string]*string)
var staticsByHost = make(map[string]map[string]*string)
var staticsByHostLock = sync.RWMutex{}

//var staticsFiles = make(map[string][]byte)
//var staticsFileByHost = make(map[string]map[string][]byte)

func resetStaticMemory() {
	staticsByHostLock.Lock()
	statics = make(map[string]*string)
	staticsByHost = make(map[string]map[string]*string)
	//staticsFiles = make(map[string][]byte)
	//staticsFileByHost = make(map[string]map[string][]byte)
	staticsByHostLock.Unlock()
}

//func SetStaticGZFile(path string, data []byte) {
//	SetStaticGZFileByHost(path, data, "")
//}
//
//func SetStaticFile(path string, data []byte) {
//	SetStaticFileByHost(path, data, "")
//}
//
//func SetStaticGZFileByHost(path string, data []byte, host string) {
//	if host == "" {
//		staticsFiles[path] = data
//	} else {
//		if staticsFileByHost[host] == nil {
//			staticsFileByHost[host] = make(map[string][]byte)
//		}
//		staticsFileByHost[host][path] = data
//	}
//}
//
//func SetStaticFileByHost(path string, data []byte, host string) {
//	var buf bytes.Buffer
//	gz := gzip.NewWriter(&buf)
//	_, _ = gz.Write(data)
//	_ = gz.Close()
//	SetStaticGZFileByHost(path, buf.Bytes(), host)
//}

func Static(path, rootPath string) {
	StaticByHost(path, rootPath, "")
}

func StaticByHost(path, rootPath, host string) {
	//rootPath = strings.ReplaceAll(rootPath, "\\", "/")
	if !filepath.IsAbs(rootPath) {
		if rootPath1, err := filepath.Abs(rootPath); err == nil {
			rootPath = rootPath1
		}
		//pos := strings.LastIndexByte(os.Args[0], '/')
		//if pos > 0 {
		//	rootPath = os.Args[0][0:pos+1] + rootPath
		//	if rootPath[0] != '/' {
		//wd, err := os.Getwd()
		//if err == nil {
		//	rootPath = filepath.Join(wd, rootPath)
		//	//rootPath = fmt.Sprintf("%s/%s", wd, rootPath)
		//}
		//	}
		//}
	}
	//if !strings.HasSuffix(path, "/") {
	//	path += "/"
	//}
	//pathSeparator := string(os.PathSeparator)
	//if !strings.HasSuffix(rootPath, pathSeparator) {
	//	rootPath += pathSeparator
	//}

	//rootPath = margePath(rootPath)
	if host == "" {
		staticsByHostLock.Lock()
		statics[path] = &rootPath
		staticsByHostLock.Unlock()
	} else {
		staticsByHostLock.Lock()
		if staticsByHost[host] == nil {
			staticsByHost[host] = make(map[string]*string)
		}
		staticsByHost[host][path] = &rootPath
		staticsByHostLock.Unlock()
	}
}

//func margePath(path string) string {
//	for strings.Index(path, "//") != -1 {
//		path = strings.ReplaceAll(path, "//", "/")
//	}
//	for strings.Index(path, "/./") != -1 {
//		path = strings.ReplaceAll(path, "/./", "/")
//	}
//	for strings.Index(path, "/../") != -1 {
//		posddg := strings.Index(path, "../")
//		path1 := path[0 : posddg-1]
//		path2 := path[posddg+3:]
//
//		pos1 := strings.LastIndexByte(path1, '/')
//		if pos1 > 0 {
//			path1 = path1[0:pos1]
//		}
//		path = fmt.Sprintf("%s/%s", path1, path2)
//	}
//	return path
//}

func findRootPath(staticConfig map[string]*string, requestPath string) (filePath string) {
	staticsByHostLock.RLock()
	fullRootPath := staticConfig[requestPath]
	staticsByHostLock.RUnlock()
	//fmt.Println(111, requestPath, fullRootPath)
	if fullRootPath != nil {
		//fmt.Println(1111, *fullRootPath)
		return *fullRootPath
	}

	staticsByHostLock.RLock()
	lastPLen := 0
	for urlPath, rootPath := range staticConfig {
		//fmt.Println(222, requestPath, urlPath)
		if strings.HasPrefix(requestPath, urlPath) {
			//fmt.Println(2222, filepath.Join(*rootPath, requestPath[len(urlPath):]))
			if len(urlPath) > lastPLen {
				lastPLen = len(urlPath)
				filePath = filepath.Join(*rootPath, requestPath[len(urlPath):])
			}
		}
	}
	staticsByHostLock.RUnlock()
	return filePath
}

func processStatic(requestPath string, request *Request, response *Response, startTime *time.Time, requestLogger *log.Logger) (bool, string) {
	baseHost := strings.SplitN(request.Host, ":", 2)[0]
	//if staticsFileByHost[request.Host] != nil && staticsFileByHost[request.Host][requestPath] != nil {
	//	fileBuf = staticsFileByHost[request.Host][requestPath]
	//} else if baseHost != request.Host && staticsFileByHost[baseHost] != nil && staticsFileByHost[baseHost][requestPath] != nil {
	//	fileBuf = staticsFileByHost[baseHost][requestPath]
	//} else if staticsFiles[requestPath] != nil {
	//	fileBuf = staticsFiles[requestPath]
	//}

	outLen := 0
	filePath := ""
	staticsByHostLock.RLock()
	staticsLen := len(statics)
	staticsByHostLen := len(staticsByHost)
	requestStaticsByFullHost := staticsByHost[request.Host]
	requestStaticsByBaseHost := staticsByHost[baseHost]
	staticsByHostLock.RUnlock()
	if staticsLen == 0 && staticsByHostLen == 0 {
		return false, "[no statics config]"
	}

	if requestStaticsByFullHost != nil && len(requestStaticsByFullHost) > 0 {
		//fmt.Println(">>>>1", requestPath, request.Host, requestStaticsByFullHost)
		filePath = findRootPath(requestStaticsByFullHost, requestPath)
	}
	if filePath == "" && baseHost != request.Host && requestStaticsByBaseHost != nil && len(requestStaticsByBaseHost) > 0 {
		//fmt.Println(">>>>2", requestPath, baseHost, requestStaticsByBaseHost)
		filePath = findRootPath(requestStaticsByBaseHost, requestPath)
	}

	//var rootPath *string
	//if requestStaticsByHost != nil {
	//	// 从虚拟主机设置中匹配
	//	rootPath = requestStaticsByHost[requestPath]
	//	if rootPath == nil && strings.ContainsRune(request.Host, ':') {
	//		fixedHost := strings.SplitN(request.Host, ":", 2)[1]
	//		staticsByHostLock.RLock()
	//		fixedStaticsByHost := staticsByHost[fixedHost]
	//		staticsByHostLock.RUnlock()
	//
	//		if fixedStaticsByHost != nil {
	//			rootPath = fixedStaticsByHost[requestPath]
	//		}
	//	}
	//}
	//// 去掉端口匹配
	//if rootPath == nil && baseHost != request.Host {
	//	staticsByHostLock.RLock()
	//	baseStaticsByHost := staticsByHost[baseHost]
	//	staticsByHostLock.RUnlock()
	//	fmt.Println(1000, baseHost, baseStaticsByHost)
	//	if baseStaticsByHost == nil {
	//
	//	}
	//	//if baseStaticsByHost != nil {
	//	//	// 从虚拟主机设置中匹配
	//	//	rootPath = baseStaticsByHost[requestPath]
	//	//	fmt.Println(10001, requestPath, rootPath)
	//	//	if rootPath == nil && strings.ContainsRune(baseHost, ':') {
	//	//		fixedHost := strings.SplitN(baseHost, ":", 2)[1]
	//	//		staticsByHostLock.RLock()
	//	//		fixedStaticsByHost := staticsByHost[fixedHost]
	//	//		staticsByHostLock.RUnlock()
	//	//		fmt.Println(10002, fixedHost, fixedStaticsByHost)
	//	//		if fixedStaticsByHost != nil {
	//	//			rootPath = fixedStaticsByHost[requestPath]
	//	//			fmt.Println(10003, requestPath, rootPath)
	//	//		}
	//	//	}
	//	//}
	//}

	if filePath == "" {
		//fmt.Println(">>>>3", requestPath, statics)
		filePath = findRootPath(statics, requestPath)
	}
	if filePath == "" {
		return false, "[no root path matched]"
	}

	//if rootPath == nil {
	//	// 从全局设置中匹配
	//	staticsByHostLock.RLock()
	//	rootPath = statics[requestPath]
	//	staticsByHostLock.RUnlock()
	//	fmt.Println(111, requestPath, rootPath)
	//	if rootPath == nil {
	//		staticsByHostLock.RLock()
	//		for p1, p2 := range statics {
	//			fmt.Println(222, requestPath, p1)
	//			if strings.HasPrefix(requestPath, p1) {
	//				rootPath = p2
	//				requestPath = requestPath[len(p1):]
	//				break
	//			}
	//		}
	//		staticsByHostLock.RUnlock()
	//	} else {
	//		//requestPath = requestPath[len(requestPath):]
	//		//fmt.Println(111, requestPath, rootPath)
	//	}
	//}
	//
	//if rootPath == nil {
	//	return false, "[no root path config]"
	//}
	//
	//filePath = *rootPath + requestPath
	//if strings.HasSuffix(filePath, string(os.PathSeparator)) {
	//	filePath += "index.html"
	//}
	info := u.GetFileInfo(filePath)
	if info != nil && info.IsDir {
		for _, indexFile := range Config.IndexFiles {
			f := filepath.Join(filePath, indexFile)
			info2 := u.GetFileInfo(f)
			if info2 != nil {
				filePath = f
				info = info2
				break
			}
		}
	}

	if info == nil {
		return false, filePath
	}

	// 不支持列出文件内容
	if info.IsDir && !Config.IndexDir {
		return false, filePath
	}

	response.Header().Set("Last-Modified", info.ModTime.UTC().Format(http.TimeFormat))
	if request.Method == http.MethodHead {
		response.Header().Set("Content-Length", u.String(info.Size))
		return true, filePath
	}

	// 检查If-Modified-Since头
	if ifModifiedSince := request.Header.Get("If-Modified-Since"); ifModifiedSince != "" {
		if t, err := time.Parse(http.TimeFormat, ifModifiedSince); err == nil {
			if !info.ModTime.After(t) {
				response.WriteHeader(http.StatusNotModified)
				return true, filePath
			}
		}
	}

	rangeStart := int64(0)
	rangeEnd := int64(0)
	useRange := false
	rangeHeader := request.Header.Get("Range")
	if rangeHeader != "" {
		if !strings.HasPrefix(rangeHeader, "bytes=") {
			http.Error(response, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
			return true, filePath
		}
		rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
		parts := strings.Split(rangeSpec, "-")
		if len(parts) != 2 {
			http.Error(response, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
			return true, filePath
		}

		start, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			http.Error(response, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
			return true, filePath
		}

		var end int64
		if parts[1] != "" {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil || end < start {
				http.Error(response, "Invalid range", http.StatusRequestedRangeNotSatisfiable)
				return true, filePath
			}
		} else {
			end = info.Size - 1
		}

		if start >= info.Size {
			http.Error(response, "Range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return true, filePath
		}

		if end >= info.Size {
			end = info.Size - 1
		}

		rangeStart = start
		rangeEnd = end
		useRange = true
	}

	// 从内存中查找
	mf := u.ReadFileFromMemory(filePath)
	var outData []byte
	if mf != nil {
		if useRange {
			outData = mf.GetData()[rangeStart : rangeEnd+1]
		} else {
			if len(outFilters) == 0 && !useRange {
				// 直接返回数据（优化内存文件性能）
				if mf.Compressed {
					response.Header().Set("Content-Encoding", "gzip")
				}

				ctype := mime.TypeByExtension(filepath.Ext(filePath))
				if ctype == "" {
					ctype = http.DetectContentType(mf.GetData()[0:512])
				}
				response.Header().Set("Content-Length", u.String(len(mf.Data)))
				response.Header().Set("Content-Type", ctype)

				// http.ServeContent(response, request, filepath.Base(filePath), serverStartTime, bytes.NewReader(mf.Data))
				response.Write(mf.Data)
				writeLog(requestLogger, "STATIC", nil, outLen, request.Request, response, nil, startTime, 0, Map{"file": filePath})
				return true, filePath
			} else {
				outData = mf.GetData()
			}
		}
	}

	if outData == nil {
		if useRange {
			if fp, err := os.Open(filePath); err == nil {
				fp.Seek(rangeStart, 0)
				outData = make([]byte, rangeEnd-rangeStart+1)
				_, _ = io.ReadFull(fp, outData)
				fp.Close()
			}
		} else {
			outData = u.ReadFileBytesN(filePath)
		}
	}
	ctype := mime.TypeByExtension(filepath.Ext(filePath))
	if ctype == "" {
		ctype = http.DetectContentType(outData[0:512])
	}
	response.Header().Set("Content-Type", ctype)

	if len(outFilters) > 0 {
		for _, filter := range outFilters {
			newResult, done := filter(map[string]any{}, request, response, outData, requestLogger)
			if newResult != nil {
				outData = u.Bytes(newResult)
			}
			if done {
				break
			}
		}
	}
	// size, err := ResponseStatic(filePath, request, response)
	// if err != nil {
	// 	return false, filePath
	// }

	outLen = len(outData)
	if Config.Compress && outLen >= Config.CompressMinSize && outLen <= Config.CompressMaxSize && strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") {
		if buf, err := u.Gzip(outData); err == nil {
			response.Header().Set("Content-Encoding", "gzip")
			outData = buf
			outLen = len(outData)
		}
	}
	response.Header().Set("Content-Length", u.String(outLen))
	response.Write(outData)

	writeLog(requestLogger, "STATIC", nil, outLen, request.Request, response, nil, startTime, 0, Map{"file": filePath})
	return true, filePath
}

// func ResponseStatic(filePath string, request *http.Request, response *Response) (int, error) {
// 	fileInfo, err := os.Stat(filePath)
// 	if err != nil {
// 		return 0, err
// 	}

// 	if len(outFilters) > 0 {
// 		// 后置过滤器
// 		buffer := bytes.Buffer{}
// 		mw := io.MultiWriter(response, &buffer)
// 		http.ServeFile(mw, request, filePath)
// 		zipWriter.Close()

// 		for _, filter := range outFilters {
// 			newResult, done := filter(args, request, response, result, requestLogger)
// 			if newResult != nil {
// 				result = newResult
// 			}
// 			if done {
// 				break
// 			}
// 		}
// 		if Config.StatisticTime {
// 			tc.Add("Out Filter")
// 		}
// 	} else {
// 		if Config.Compress && int(fileInfo.Size()) >= Config.CompressMinSize && int(fileInfo.Size()) <= Config.CompressMaxSize && strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") {
// 			zipWriter := NewGzipResponseWriter(response)
// 			http.ServeFile(zipWriter, request, filePath)
// 			zipWriter.Close()
// 		} else {
// 			http.ServeFile(response, request, filePath)
// 		}
// 	}
// 	return int(fileInfo.Size()), nil
// }
