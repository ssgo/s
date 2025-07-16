package s

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	"github.com/ssgo/redis"
	"github.com/ssgo/u"
)

var uidServerDate string
var uidServerStart int64
var uidServerIndex int64 = -1
var uidSec int
var uidIndexes = map[int]map[uint]bool{}

var uidLock = sync.Mutex{}
var uidShutdownHookSet = false

var fileLocksLock = sync.Mutex{}
var fileLocks = map[string]*sync.Mutex{}

func resetUtilityMemory() {
	uidServerDate = ""
	uidServerStart = 0
	uidServerIndex = -1
	uidSec = 0
	uidIndexes = map[int]map[uint]bool{}
	uidShutdownHookSet = false
	fileLocks = map[string]*sync.Mutex{}
}

func trySetServerId(rdConn redigo.Conn, hkey string, sid int64) (bool, error) {
	if rdConn != nil {
		r, err := rdConn.Do("HSETNX", hkey, sid, true)
		if err == nil {
			if i, ok := r.(int64); ok && i == 1 {
				return true, nil
			}
		}
		return false, err
	} else {
		return false, nil
	}
}

func uniqueId() []byte {
	tm := time.Now()
	date := tm.Format("0102")
	// 检查每天重新排列的服务器编号
	if date != uidServerDate {
		rd1 := getRedis1()
		var rdConn1 redigo.Conn
		if rd1 != nil {
			rdConn1, _ = rd1.GetConnection()
		}
		makeServerIndexTimes := 0
		makeServerIndexOk := false
		uidLock.Lock()
		if date != uidServerDate {
			uidServerDate = date
			uidServerStart = tm.Unix()
			if rdConn1 == nil {
				// 尝试环境变量中指定的ServerId
				if !makeServerIndexOk && os.Getenv("SERVER_ID") != "" {
					uidServerIndex = u.Int64(os.Getenv("SERVER_ID"))
					if uidServerIndex >= 0 && uidServerIndex < 238328 {
						makeServerIndexOk = true
						logInfo("get server id for unique id over env", "uidServerId", uidServerIndex)
					}
				}

				// 尝试文件中保存的ServerId
				if !makeServerIndexOk && u.FileExists(".server_id") {
					serverIdInFile, err := u.ReadFile(".server_id")
					if err == nil {
						uidServerIndex = u.Int64(serverIdInFile)
						if uidServerIndex >= 0 && uidServerIndex < 238328 {
							makeServerIndexOk = true
							logInfo("get server id for unique id over file", "uidServerId", uidServerIndex, "uidFile", serverIdInFile)
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
				if rdConn1 != nil {
					r, err := rdConn1.Do("EXISTS", hkey)
					if err == nil {
						i, ok := r.(int64)
						hexists = ok && i == 1
					}
				}

				// 先尝试沿用旧ID
				if uidServerIndex >= 0 {
					if ok, _ := trySetServerId(rdConn1, hkey, uidServerIndex); ok {
						makeServerIndexOk = true
						logInfo("get server id for unique id over old", "uidServerId", uidServerIndex)
					}
				}

				// 尝试环境变量中指定的ServerId
				if !makeServerIndexOk && os.Getenv("SERVER_ID") != "" {
					uidServerIndex = u.Int64(os.Getenv("SERVER_ID"))
					if uidServerIndex >= 0 && uidServerIndex < 238328 {
						if ok, _ := trySetServerId(rdConn1, hkey, uidServerIndex); ok {
							makeServerIndexOk = true
							logInfo("get server id for unique id over env", "uidServerId", uidServerIndex)
						}
					}
				}

				// 尝试文件中保存的ServerId
				if !makeServerIndexOk && u.FileExists(".server_id") {
					serverIdInFile, err := u.ReadFile(".server_id")
					if err == nil {
						uidServerIndex = u.Int64(serverIdInFile)
						if uidServerIndex >= 0 && uidServerIndex < 238328 {
							if ok, _ := trySetServerId(rdConn1, hkey, uidServerIndex); ok {
								makeServerIndexOk = true
								logInfo("get server id for unique id over file", "uidServerId", uidServerIndex, "uidFile", serverIdInFile)
							}
						}
					}
				}

				// 如果沿用旧ID失败，尝试碰撞100次找到新的空闲索引
				if !makeServerIndexOk {
					for makeServerIndexTimes = 0; makeServerIndexTimes < 100; makeServerIndexTimes++ {
						uidServerIndex = u.GlobalRand1.Int63n(238328)
						if ok, err := trySetServerId(rdConn1, hkey, uidServerIndex); ok {
							makeServerIndexOk = true
							_ = u.WriteFile(".server_id", u.String(uidServerIndex))
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
					if rdConn1 != nil {
						indexKey := fmt.Sprint("USI", date, "Index")
						for {
							makeServerIndexTimes++
							r, err := rdConn1.Do("INCR", indexKey)
							if err == nil {
								if i, ok := r.(int64); ok {
									if uidServerIndex >= 238328 {
										break
									}
									if i == 1 {
										// 第一次创建累加器，设置过期
										_, _ = rdConn1.Do("EXPIRE", indexKey, 86400)
									}
									uidServerIndex = i
									if ok, err := trySetServerId(rdConn1, hkey, uidServerIndex); ok {
										makeServerIndexOk = true
										_ = u.WriteFile(".server_id", u.String(uidServerIndex))
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
				}

				if !makeServerIndexOk {
					uidServerIndex = u.GlobalRand1.Int63n(238328)
				}

				// 第一次创建Hash，设置过期
				if !hexists && rdConn1 != nil {
					_, _ = rdConn1.Do("EXPIRE", hkey, 86400)
				}

				if makeServerIndexOk && !uidShutdownHookSet {
					uidShutdownHookSet = true
					AddShutdownHook(func() {
						rd1 := getRedis1()
						if rd1 != nil {
							rd1.HDEL("USI"+uidServerDate, u.String(uidServerIndex))
						}
					})
				}
			}
		}
		uidLock.Unlock()
		if rdConn1 != nil {
			_ = rdConn1.Close()
			rdConn1 = nil
		}

		if !makeServerIndexOk {
			if rd1 != nil {
				ServerLogger.Error("failed to make unique id server index", "times", makeServerIndexTimes)
			}
		} else if makeServerIndexTimes >= 100 {
			ServerLogger.Warning("make unique id server index slowly", "times", makeServerIndexTimes)
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
		ServerLogger.Warning("failed to make unique id second index，use random unique id", "times", makeSecIndexTimes, "second", uidSec, "indexSize", len(uidIndexes[uidSec]), "randomUid", string(uid))
		return uid
	} else if makeSecIndexTimes >= 1000 {
		ServerLogger.Warning("make unique id second index slowly", "times", makeSecIndexTimes, "second", uidSec, "indexSize", len(uidIndexes[uidSec]))
	}

	// 添加序号
	uid := u.AppendInt(nil, uint64(secIndex))

	// 添加时间戳
	sec1 := u.Bytes(tm.Unix())
	secLen := len(sec1)
	sec2 := make([]byte, secLen+1)
	for i := 0; i < secLen; i++ {
		sec2[i+1] = sec1[secLen-i-1]
	}
	sec2[0] = byte(u.Int(u.GlobalRand2.Intn(10)) + 48)
	timeStr := u.AppendInt(nil, u.Uint64(sec2))
	for len(timeStr) < 7 {
		timeStr = append(timeStr, '9')
	}
	uid = append(uid, timeStr...)

	// 添加服务器序号
	serverIndexStr := u.AppendInt(nil, uint64(uidServerIndex))
	for len(serverIndexStr) < 3 {
		serverIndexStr = append(serverIndexStr, '9')
	}
	uid = append(uid, serverIndexStr...)
	return uid
}

func catUniqueId(size int) string {
	id := uniqueId()
	if len(id) > size {
		return string(id[0:size])
	}

	for i := size - len(id); i > 0; i-- {
		var c int
		if i%2 == 0 {
			c = u.GlobalRand1.Intn(62)
		} else {
			c = u.GlobalRand2.Intn(62)
		}
		id = append(id, u.EncodeInt(uint64(c))[0])
	}

	return string(id)
}

func UniqueId() string {
	return string(uniqueId())
}

func UniqueId12() string {
	return catUniqueId(12)
}

func UniqueId14() string {
	return catUniqueId(14)
}

func UniqueId16() string {
	return catUniqueId(16)
}

func UniqueId20() string {
	return catUniqueId(20)
}

func Id6(space string) string {
	return makeId(space, u.Id6)
}

func Id8(space string) string {
	return makeId(space, u.Id8)
}

func Id10(space string) string {
	return makeId(space, u.Id10)
}

func Id12(space string) string {
	return makeId(space, u.Id12)
}

func Id6L(space string) string {
	return makeId(space, makeId6L)
}

func Id8L(space string) string {
	return makeId(space, makeId8L)
}

func Id10L(space string) string {
	return makeId(space, makeId10L)
}

func Id12L(space string) string {
	return makeId(space, makeId12L)
}

func makeId6L() string {
	return strings.ToLower(u.Id6())
}

func makeId8L() string {
	return strings.ToLower(u.Id8())
}

func makeId10L() string {
	return strings.ToLower(u.Id10())
}

func makeId12L() string {
	return strings.ToLower(u.Id12())
}

// 分配唯一编号
func makeId(space string, idMaker func() string) string {
	var rd1 *redis.Redis
	if Config.IdServer != "" {
		rd1 = redis.GetRedis(Config.IdServer, ServerLogger)
	}
	var id string
	for i := 0; i < 10000; i++ {
		id = idMaker()
		key := fmt.Sprint("ID", space, id[0:2])
		field := id[2:]
		if rd1 != nil {
			if rd1.HEXISTS(key, field) {
				continue
			} else {
				rd1.HSET(key, field, "")
				return id
			}
		} else {
			idFile := filepath.Join("data", "ids", id[0:2], id[2:4], id)
			if u.FileExists(idFile) {
				continue
			} else {
				_ = u.WriteFile(idFile, "")
				return id
			}
		}
	}
	return id
}
