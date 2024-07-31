package s

import (
	"github.com/ssgo/discover"
	"sync"
	"time"
)

type memoryCache struct {
	value     any
	seconds   int64
	cacheTime int64
	locker    sync.Mutex
}

var memoryCaches = map[string]*memoryCache{}
var memoryCacheLocker = sync.Mutex{}
var memoryCacheStarted bool

func CacheByMemory(key string, seconds int64, maker func() any) any {
	memoryCacheLocker.Lock()
	cache := memoryCaches[key]
	if cache == nil {
		cache = &memoryCache{
			value:     nil,
			seconds:   0,
			cacheTime: 0,
			locker:    sync.Mutex{},
		}
		memoryCaches[key] = cache
	}
	memoryCacheLocker.Unlock()

	cache.locker.Lock()
	value := cache.value
	nowTime := time.Now().Unix()
	//fmt.Println("   >>>>>>>>>", value == nil, nowTime-cache.cacheTime, seconds)
	if value == nil || nowTime-cache.cacheTime > seconds {
		// 需要生成缓存
		value = maker()
		cache.value = value
		cache.seconds = seconds
		cache.cacheTime = time.Now().Unix()
	}
	cache.locker.Unlock()

	return value
}

func StartMemoryCacheCleaner() {
	NewTimerServer("memoryCacheCleaner", time.Minute, func(isRunning *bool) {
		cleanList := make([]*memoryCache, 0)
		nowTime := time.Now().Unix()
		memoryCacheLocker.Lock()
		for _, cache := range memoryCaches {
			if cache.value != nil && nowTime-cache.cacheTime > cache.seconds {
				cleanList = append(cleanList, cache)
			}
		}
		memoryCacheLocker.Unlock()

		if !*isRunning {
			return
		}

		for _, cache := range cleanList {
			cache.locker.Lock()
			//fmt.Println("   !!!!!>>>>", cache.value != nil, nowTime-cache.cacheTime, cache.seconds)
			if cache.value != nil && nowTime-cache.cacheTime > cache.seconds {
				cache.value = nil
				cache.cacheTime = 0
			}
			cache.locker.Unlock()

			if !*isRunning {
				return
			}
		}
	}, func() {
		Subscribe("S_ClearMemoryCache_"+discover.Config.App, func() {
			// 连接重置时清除所有缓存
			//fmt.Println("######## 2.1")
			clearAllMemoryCache()
		}, func(msgBytes []byte) {
			clearMemoryCache(string(msgBytes))
		})
		memoryCacheStarted = true
	}, func() {
		//fmt.Println("######## 2.2")
		clearAllMemoryCache()
	})
}

func ClearMemoryCache(key string) {
	if memoryCacheStarted {
		// 清除服务启动时，通过订阅通知进行缓存清理
		//fmt.Println("######## Publish ", key)
		Publish("S_ClearMemoryCache_"+discover.Config.App, key)
	} else {
		clearMemoryCache(key)
	}
}

func clearMemoryCache(key string) {
	//fmt.Println("######## 1")
	memoryCacheLocker.Lock()
	cache := memoryCaches[key]
	memoryCacheLocker.Unlock()

	if cache == nil {
		return
	}

	cache.locker.Lock()
	cache.value = nil
	cache.cacheTime = 0
	cache.locker.Unlock()
}

func clearAllMemoryCache() {
	memoryCacheLocker.Lock()
	memoryCaches = map[string]*memoryCache{}
	memoryCacheLocker.Unlock()
}
