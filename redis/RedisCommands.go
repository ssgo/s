package redis

func StringsToInterfaces(in []string) []interface{} {
	a := make([]interface{}, len(in))
	for i, v := range in {
		a[i] = v
	}
	return a
}

func (this *Redis) DEL(keys ...string) int {
	return this.Do("DEL", StringsToInterfaces(keys)...).Int()
}
func (this *Redis) EXISTS(key string) bool {
	return this.Do("EXISTS", key).Bool()
}
func (this *Redis) EXPIRE(key string, seconds int) bool {
	if seconds > 315360000 {
		return this.Do("EXPIREAT", key).Bool()
	} else {
		return this.Do("EXISTS", key).Bool()
	}
}
func (this *Redis) KEYS(patten string) []string {
	return this.Do("KEYS", patten).Strings()
}

func (this *Redis) GET(key string) *Result {
	return this.Do("GET", key)
}
func (this *Redis) SET(key string, value interface{}) bool {
	return this.Do("SET", key, value).Bool()
}
func (this *Redis) SETEX(key string, seconds int, value interface{}) bool {
	return this.Do("SETEX", key, seconds, value).Bool()
}
func (this *Redis) SETNX(key string, value interface{}) bool {
	return this.Do("SETNX", key, value).Bool()
}
func (this *Redis) GETSET(key string, value interface{}) *Result {
	return this.Do("GETSET", key, value)
}

func (this *Redis) INCR(key string) int64 {
	return this.Do("INCR", key).Int64()
}
func (this *Redis) DECR(key string) int64 {
	return this.Do("DECR", key).Int64()
}

func (this *Redis) MGET(keys ...string) []Result {
	return this.Do("MGET", StringsToInterfaces(keys)...).Results()
}
func (this *Redis) MSET(keyAndValues ...interface{}) bool {
	return this.Do("MSET", keyAndValues...).Bool()
}

func (this *Redis) HGET(key, field string) *Result {
	return this.Do("HGET", key, field)
}
func (this *Redis) HSET(key, field string, value interface{}) bool {
	return this.Do("HSET", key, field, value).Bool()
}
func (this *Redis) HSETNX(key, field string, value interface{}) bool {
	return this.Do("HSETNX", key, field, value).Bool()
}
func (this *Redis) HMGET(key string, fields ...string) []Result {
	return this.Do("HMGET", append(append([]interface{}{}, key), StringsToInterfaces(fields)...)...).Results()
}
func (this *Redis) HGETALL(key string) map[string]*Result {
	return this.Do("HGETALL", key).ResultMap()
}
func (this *Redis) HMSET(key string, fieldAndValues ...interface{}) bool {
	return this.Do("HMSET", append(append([]interface{}{}, key), fieldAndValues...)...).Bool()
}
func (this *Redis) HKEYS(key string) []string {
	return this.Do("HKEYS", key).Strings()
}
func (this *Redis) HLEN(key string) int {
	return this.Do("HLEN", key).Int()
}
func (this *Redis) HDEL(key string, fields ...string) int {
	return this.Do("HDEL", append(append([]interface{}{}, key), StringsToInterfaces(fields)...)...).Int()
}
func (this *Redis) HEXISTS(key, field string) bool {
	return this.Do("HEXISTS", key, field).Bool()
}
func (this *Redis) HINCR(key, field string) int64 {
	return this.Do("HINCRBY", key, field, 1).Int64()
}
func (this *Redis) HDECR(key, field string) int64 {
	return this.Do("HDECRBY", key, field, 1).Int64()
}

func (this *Redis) LPUSH(key string, values ...string) int {
	return this.Do("LPUSH", append(append([]interface{}{}, key), StringsToInterfaces(values)...)...).Int()
}
func (this *Redis) RPUSH(key string, values ...string) int {
	return this.Do("RPUSH", append(append([]interface{}{}, key), StringsToInterfaces(values)...)...).Int()
}
func (this *Redis) LPOP(key string) *Result {
	return this.Do("LPOP", key)
}
func (this *Redis) RPOP(key string) *Result {
	return this.Do("RPOP", key)
}
func (this *Redis) LLEN(key string) int {
	return this.Do("LLEN", key).Int()
}
func (this *Redis) LRANGE(key string, start, stop int) []Result {
	return this.Do("LRANGE", key, start, stop).Results()
}
