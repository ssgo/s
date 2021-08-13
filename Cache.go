package s

import (
	"sync"
	"time"
)

type memoryCache struct {
	value     interface{}
	seconds   int64
	cacheTime int64
	locker    sync.Mutex
}

var memoryCaches = map[string]*memoryCache{}
var memoryCacheLocker = sync.Mutex{}

func CacheByMemory(key string, seconds int64, maker func() interface{}) interface{} {
	memoryCacheLocker.Lock()
	cache := memoryCaches[key]
	if cache == nil {
		cache = &memoryCache{
			value:     nil,
			seconds:   0,
			cacheTime: 0,
			locker:    sync.Mutex{},
		}
	}
	memoryCacheLocker.Unlock()

	cache.locker.Lock()
	value := cache.value
	nowTime := time.Now().Unix()
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
	NewTimerServer("memoryCacheCleaner", time.Minute*5, func(isRunning *bool) {
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
			if cache.value != nil && nowTime-cache.cacheTime > cache.seconds {
				cache.value = nil
				cache.cacheTime = 0
			}
			cache.locker.Unlock()

			if !*isRunning {
				return
			}
		}
	}, nil, nil)
}

func ClearMemoryCache(key string) {
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
