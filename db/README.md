# Go语言的一个数据库操作封装
核心思想是传入一个结果容器，根据容器的类型自动填充数据，方便使用

## 配置

```json
{
  "test": {
    "type": "mysql",
    "user": "test",
    "password": "34RVCy0rQBSQmLX64xjoyg==",	// 使用 github.com/ssgo/base/passwordMaker 生成
    "host": "/tmp/mysql.sock",
    "db": "test",
    "maxOpens": 100,	// 最大连接数，0表示不限制
    "maxIdles": 30,		// 最大空闲连接，0表示不限制
    "maxLiftTime": 0	// 每个连接的存活时间，0表示永远
  }
}
```



## API

```go
import github.com/ssgo/db

// 自定义加密密钥，
func SetEncryptKeys(key, iv []byte){}

// 获得一个数据库操作实例，这是一个连接池，直接操作即可不需要实例化
func GetDB(name string) (*DB, error){}


// 释放数据库操作实例，正常情况下不应该操作，否则整个连接池都将无法使用
func (this *DB) Destroy() error{}

// 取得原始的 sql.DB 对象，可自行操作
func (this *DB) GetConnection() *sql.DB{}

// 查询数据，根据接收结果的对象不同执行不同的查询
// results := make([]userInfo, 0)               取全部结果，存为对象
// results := make([]map[string]interface{}, 0) 取全部结果，保留原始类型
// results := make([]map[string]string, 0)      取全部结果，统一转换为string
// results := make([]map[string]int, 0)         取全部结果，统一转换为int，不可转换会报错
// results := make([][]string, 0)               取全部结果，存为数组
// results := userInfo{}                        取第一行数据，存为对象
// results := map[string]interface{}{}          取第一行数据，存为map
// results := make([]string, 0)                 取全部第一列数据
// var results int                              取第一行第一列数据
func (this *DB) Query(results interface{}, requestSql string, args ...interface{}) error {}

// 执行普通查询，返回影响列数
func (this *DB) Exec(requestSql string, args ...interface{}) (int64, error) {}

// 执行普通INSERT或REPLACE，返回lastInsertId
func (this *DB) ExecInsert(requestSql string, args ...interface{}) (int64, error) {}

// 按数据对象自动生成INSERT语句并执行，data支持Map和Struct
func (this *DB) Insert(table string, data interface{}) (int64, error) {}

// 按数据对象自动生成REPLACE语句并执行，data支持Map和Struct
func (this *DB) Replace(table string, data interface{}) (int64, error) {}

// 按数据对象自动生成UPDATE语句并执行，data支持Map和Struct
func (this *DB) Update(table string, data interface{}, wheres string, args ...interface{}) (int64, error) {}

// 开启一个事务
func (this *DB) Begin() (*Tx, error) {}

// 预处理
func (this *DB) Prepare(requestSql string) (*Stmt, error) {}


// 在事务中操作，同(this *DB)
func (this *Tx) Query(results interface{}, requestSql string, args ...interface{}) error {}
func (this *Tx) Exec(requestSql string, args ...interface{}) (int64, error) {}
func (this *Tx) ExecInsert(requestSql string, args ...interface{}) (int64, error) {}
func (this *Tx) Insert(table string, data interface{}) (int64, error) {}
func (this *Tx) Replace(table string, data interface{}) (int64, error) {}
func (this *Tx) Update(table string, data interface{}, wheres string, args ...interface{}) (int64, error) {}
func (this *Tx) Prepare(requestSql string) (*Stmt, error) {

// 提交
func (this *Tx) Commit() error {

// 回滚
func (this *Tx) Rollback() error {


// 批量执行，返回受影响的列数
func (this *Stmt) Exec(args ...interface{}) (int64, error) {}

// 批量执行，返回lastInsertId
func (this *Stmt) ExecInsert(args ...interface{}) (int64, error) {}

// 关闭预处理
func (this *Stmt) Close() error {}

```



## 运行测试用例

#### 把 tests/db.json.sample 复制为 tests/db.json

修改你本地数据库连接信息

使用 github.com/ssgo/base 中的 passwordMaker 创建加密后的密码写入 db.json 中的 password

进入 tests 目录运行 go test



## 项目中的配置

#### 把 /db.json.sample 复制到你项目中命名为 /db.json

修改你本地数据库连接信息

使用 passwordMaker 创建加密后的密码写入 db.json 中的 password

### 自定义加密（强烈推荐）

##### 把 /dbInit.go.sample 复制到你项目中命名为 /dbInit.go

修改其中的 key 和 iv 的内容，长度至少32字节，保持与passwordMaker 中一致

也可以以其他方式只要在 init 函数中调用 db.SetEncryptKeys 设置匹配的 key和iv即可

### 