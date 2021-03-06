package s

import (
	"fmt"
	redigo "github.com/gomodule/redigo/redis"
	"github.com/ssgo/discover"
	"github.com/ssgo/redis"
	"github.com/ssgo/u"
	"os"
	"sync"
	"time"
)

var uidServerDate string
var uidServerStart int64
var uidServerIndex int64 = -1
var uidSec int
var uidIndexes = map[int]map[uint]bool{}

var uidLock = sync.Mutex{}
var uidShutdownHookSet = false

func trySetServerId(rdConn redigo.Conn, hkey string, sid int64) (bool, error) {
	r, err := rdConn.Do("HSETNX", hkey, sid, true)
	if err == nil {
		if i, ok := r.(int64); ok && i == 1 {
			return true, nil
		}
	}
	return false, err
}

func uniqueId() []byte {
	tm := time.Now()
	date := tm.Format("0102")
	// 检查每天重新排列的服务器编号
	if date != uidServerDate {
		if _rd == nil {
			_rd = redis.GetRedis(discover.Config.Registry, serverLogger)
		}

		rdConn := _rd.GetConnection()
		makeServerIndexTimes := 0
		makeServerIndexOk := false
		uidLock.Lock()
		if date != uidServerDate {
			uidServerDate = date
			uidServerStart = tm.Unix()
			if rdConn == nil {
				// 先尝试沿用旧ID
				hkey := "USI" + date
				if uidServerIndex >= 0 {
					if ok, _ := trySetServerId(rdConn, hkey, uidServerIndex); ok {
						makeServerIndexOk = true
						logInfo("get server id for unique id over old", "uidServerId", uidServerIndex)
					}
				}

				// 尝试环境变量中指定的ServerId
				if !makeServerIndexOk && os.Getenv("SERVER_ID") != "" {
					uidServerIndex = u.Int64(os.Getenv("SERVER_ID"))
					if uidServerIndex >= 0 && uidServerIndex < 238328 {
						if ok, _ := trySetServerId(rdConn, hkey, uidServerIndex); ok {
							makeServerIndexOk = true
							logInfo("get server id for unique id over env", "uidServerId", uidServerIndex)
						}
					}
				}

				// 尝试文件中保存的ServerId
				if !makeServerIndexOk && u.FileExists("/tmp/server_id") {
					serverIdInFile, err := u.ReadFile("/tmp/server_id", 6)
					if err == nil {
						uidServerIndex = u.Int64(serverIdInFile)
						if uidServerIndex >= 0 && uidServerIndex < 238328 {
							if ok, _ := trySetServerId(rdConn, hkey, uidServerIndex); ok {
								makeServerIndexOk = true
								logInfo("get server id for unique id over file", "uidServerId", uidServerIndex)
							}
						}
					}
				}

				if !makeServerIndexOk {
					uidServerIndex = u.GlobalRand1.Int63n(238328)
				}
			} else {
				// 检查Hash
				hkey := "USI" + date
				hexists := false
				r, err := rdConn.Do("EXISTS", hkey)
				if err == nil {
					i, ok := r.(int64)
					hexists = ok && i == 1
				}

				// 先尝试沿用旧ID
				if uidServerIndex >= 0 {
					if ok, _ := trySetServerId(rdConn, hkey, uidServerIndex); ok {
						makeServerIndexOk = true
						logInfo("get server id for unique id over old", "uidServerId", uidServerIndex)
					}
				}

				// 尝试环境变量中指定的ServerId
				if !makeServerIndexOk && os.Getenv("SERVER_ID") != "" {
					uidServerIndex = u.Int64(os.Getenv("SERVER_ID"))
					if uidServerIndex >= 0 && uidServerIndex < 238328 {
						if ok, _ := trySetServerId(rdConn, hkey, uidServerIndex); ok {
							makeServerIndexOk = true
							logInfo("get server id for unique id over env", "uidServerId", uidServerIndex)
						}
					}
				}

				// 尝试文件中保存的ServerId
				if !makeServerIndexOk && u.FileExists("/tmp/server_id") {
					serverIdInFile, err := u.ReadFile("/tmp/server_id", 6)
					if err == nil {
						uidServerIndex = u.Int64(serverIdInFile)
						if uidServerIndex >= 0 && uidServerIndex < 238328 {
							if ok, _ := trySetServerId(rdConn, hkey, uidServerIndex); ok {
								makeServerIndexOk = true
								logInfo("get server id for unique id over file", "uidServerId", uidServerIndex)
							}
						}
					}
				}

				// 如果沿用旧ID失败，尝试碰撞100次找到新的空闲索引
				if !makeServerIndexOk {
					for makeServerIndexTimes = 0; makeServerIndexTimes < 100; makeServerIndexTimes++ {
						uidServerIndex = u.GlobalRand1.Int63n(238328)
						if ok, err := trySetServerId(rdConn, hkey, uidServerIndex); ok {
							makeServerIndexOk = true
							_ = u.WriteFile("/tmp/server_id", u.String(uidServerIndex))
							logInfo("get server id for unique id over hit", "uidServerId", uidServerIndex)
							break
						} else if err != nil {
							break
						}
					}
				}

				// 如果尝试100次碰撞失败，使用累加器来获得空闲索引
				if !makeServerIndexOk {
					// 1000次随机没有命中的话，使用计数器顺序使用
					indexKey := fmt.Sprint("USI", date, "Index")
					for {
						makeServerIndexTimes++
						r, err := rdConn.Do("INCR", indexKey)
						if err == nil {
							if i, ok := r.(int64); ok {
								if uidServerIndex >= 238328 {
									break
								}
								if i == 1 {
									// 第一次创建累加器，设置过期
									_, _ = rdConn.Do("EXPIRE", indexKey, 86400)
								}
								uidServerIndex = i
								if ok, err := trySetServerId(rdConn, hkey, uidServerIndex); ok {
									makeServerIndexOk = true
									_ = u.WriteFile("/tmp/server_id", u.String(uidServerIndex))
									logInfo("get server id for unique id over incr", "uidServerId", uidServerIndex)
									break
								} else if err != nil {
									break
								}
							} else {
								break
							}
						} else {
							break
						}
					}
				}

				if !makeServerIndexOk {
					uidServerIndex = u.GlobalRand1.Int63n(238328)
				}

				// 第一次创建Hash，设置过期
				if !hexists {
					_, _ = rdConn.Do("EXPIRE", hkey, 86400)
				}

				if makeServerIndexOk && !uidShutdownHookSet {
					uidShutdownHookSet = true
					AddShutdownHook(func() {
						_rd.HDEL("USI"+uidServerDate, u.String(uidServerIndex))
					})
				}
			}
		}
		uidLock.Unlock()
		if rdConn != nil {
			_ = rdConn.Close()
		}

		if !makeServerIndexOk {
			serverLogger.Error("failed to make unique id server index", "times", makeServerIndexTimes)
		} else if makeServerIndexTimes >= 100 {
			serverLogger.Warning("make unique id server index slowly", "times", makeServerIndexTimes)
		}
	}

	// 检查秒内的索引值，换秒后重新计数
	var secIndex uint
	sec := int(tm.Unix() - uidServerStart)
	makeSecIndexTimes := 0
	makeSecIndexOk := false
	uidLock.Lock()
	if uidSec != sec {
		// 清除多余的数据
		for k := range uidIndexes {
			if k != uidSec {
				delete(uidIndexes, k)
			}
		}
		// 创建新的每秒索引容器
		uidIndexes[sec] = map[uint]bool{}
		uidSec = sec
		//	uidSecIndex = 0
	}
	if uidIndexes[sec] == nil {
		uidIndexes[sec] = map[uint]bool{}
	}
	if len(uidIndexes[sec]) < 200000 {
		for makeSecIndexTimes = 0; makeSecIndexTimes < 10000; makeSecIndexTimes++ {
			secIndex = uint(u.GlobalRand2.Int63n(238328))
			if !uidIndexes[sec][secIndex] {
				uidIndexes[sec][secIndex] = true
				makeSecIndexOk = true
				break
			}
		}
	}
	uidLock.Unlock()

	if !makeSecIndexOk {
		uid := u.AppendInt(nil, uint64(u.GlobalRand1.Intn(56800235583)))
		uid = u.AppendInt(uid, uint64(u.GlobalRand1.Intn(56800235583)))
		for len(uid) < 11 {
			uid = u.AppendInt(uid, uint64(u.GlobalRand1.Intn(56800235583)))
		}
		if len(uid) > 11 {
			uid = uid[0:11]
		}
		serverLogger.Warning("failed to make unique id second index，use random unique id", "times", makeSecIndexTimes, "second", uidSec, "indexSize", len(uidIndexes[uidSec]), "randomUid", string(uid))
		return uid
	} else if makeSecIndexTimes >= 1000 {
		serverLogger.Warning("make unique id second index slowly", "times", makeSecIndexTimes, "second", uidSec, "indexSize", len(uidIndexes[uidSec]))
	}

	// 添加服务器序号
	uid := u.AppendInt(nil, uint64(uidServerIndex))
	for len(uid) < 3 {
		uid = append(uid, '9')
	}

	// 添加时间戳
	uid = u.AppendInt(uid, uint64(tm.Unix()-946656000))

	// 添加序号
	uid = u.AppendInt(uid, uint64(secIndex))
	for len(uid) < 11 {
		uid = append(uid, '9')
	}

	return uid
}

func UniqueId() string {
	return string(uniqueId())
}

func UniqueIdX() string {
	buf := u.AppendInt(nil, uint64(u.GlobalRand1.Intn(62)))
	buf = u.AppendInt(buf, uint64(u.GlobalRand2.Intn(62)))
	buf = append(buf, uniqueId()...)
	buf = u.AppendInt(buf, uint64(u.GlobalRand1.Intn(62)))
	buf = u.AppendInt(buf, uint64(u.GlobalRand2.Intn(62)))
	buf = u.AppendInt(buf, uint64(u.GlobalRand1.Intn(62)))
	return string(buf)
}
