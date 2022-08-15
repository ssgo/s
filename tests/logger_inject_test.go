package tests

import (
	"github.com/ssgo/redis"
	"github.com/ssgo/standard"
	"github.com/ssgo/u"
	"net/http"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/ssgo/log"
	"github.com/ssgo/s"
)

type TestLoggerInjectResult struct {
	TraceId   string
	RequestId string
}

func TestGet(tt *testing.T) {
	t := s.T(tt)

	s.Register(0, "/redis", func(request *http.Request, logger *log.Logger) TestLoggerInjectResult {
		rd := redis.GetRedis("test", logger)
		return TestLoggerInjectResult{
			TraceId:   rd.GetLogger().GetTraceId(),
			RequestId: request.Header.Get(standard.DiscoverHeaderRequestId),
		}
	}, "")

	//s.Register(0, "/db", func(request *http.Request, logger *log.Logger) TestLoggerInjectResult {
	//	db := db.GetDB("test", logger)
	//	return TestLoggerInjectResult{
	//		TraceId:   db.GetLogger().GetTraceId(),
	//		RequestId: request.Header.Get(standard.DiscoverHeaderRequestId),
	//	}
	//})

	as := s.AsyncStart()
	defer as.Stop()

	r := TestLoggerInjectResult{}

	_ = as.Get("/redis").To(&r)
	t.Test(r.TraceId != "" && r.TraceId == r.RequestId, "TestGet redis failed", u.String(r))

	_ = as.Get("/db").To(&r)
	t.Test(r.TraceId != "" && r.TraceId == r.RequestId, "TestGet db failed", u.String(r))
}

func TestInject(tt *testing.T) {
	t := s.T(tt)

	type RedisA = *redis.Redis
	s.SetInject(RedisA(redis.GetRedis("test", nil)))

	//type DBA = *db.DB
	//s.SetInject(DBA(db.GetDB("test", nil)))

	s.Register(0, "/redis", func(request *http.Request, rd RedisA) TestLoggerInjectResult {
		return TestLoggerInjectResult{
			TraceId:   rd.GetLogger().GetTraceId(),
			RequestId: request.Header.Get(standard.DiscoverHeaderRequestId),
		}
	}, "")

	//s.Register(0, "/db", func(request *http.Request, db DBA) TestLoggerInjectResult {
	//	return TestLoggerInjectResult{
	//		TraceId:   db.GetLogger().GetTraceId(),
	//		RequestId: request.Header.Get(standard.DiscoverHeaderRequestId),
	//	}
	//})

	as := s.AsyncStart()
	defer as.Stop()

	r := TestLoggerInjectResult{}

	_ = as.Get("/redis").To(&r)
	t.Test(r.TraceId != "" && r.TraceId == r.RequestId, "TestInject redis failed", u.String(r))

	_ = as.Get("/db").To(&r)
	t.Test(r.TraceId != "" && r.TraceId == r.RequestId, "TestInject db failed", u.String(r))
}

type Context struct {
	s.Context
}

//func (ctx *Context) GetDB() *db.DB {
//	return db.GetDB("default", ctx.Logger)
//}
//
//func (ctx *Context) GetUserDB() *db.DB {
//	return db.GetDB("user", ctx.Logger)
//}

func (ctx *Context) GetRedis() *redis.Redis {
	return redis.GetRedis("test", ctx.Logger)
}

func TestInjectContext(tt *testing.T) {
	t := s.T(tt)

	s.SetInject(&Context{})

	s.Register(0, "/redis", func(request *http.Request, ctx *Context) TestLoggerInjectResult {
		return TestLoggerInjectResult{
			TraceId:   ctx.GetRedis().GetLogger().GetTraceId(),
			RequestId: request.Header.Get(standard.DiscoverHeaderRequestId),
		}
	}, "")

	//s.Register(0, "/db", func(request *http.Request, ctx *Context) TestLoggerInjectResult {
	//	return TestLoggerInjectResult{
	//		TraceId:   ctx.GetUserDB().GetLogger().GetTraceId(),
	//		RequestId: request.Header.Get(standard.DiscoverHeaderRequestId),
	//	}
	//})

	as := s.AsyncStart()
	defer as.Stop()

	r := TestLoggerInjectResult{}

	_ = as.Get("/redis").To(&r)
	t.Test(r.TraceId != "" && r.TraceId == r.RequestId, "TestInjectContext redis failed", u.String(r))

	_ = as.Get("/db").To(&r)
	t.Test(r.TraceId != "" && r.TraceId == r.RequestId, "TestInjectContext db failed", u.String(r))
}
