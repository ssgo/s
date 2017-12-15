# Go语言的一个数据库操作封装
核心思想是传入一个结果容器，根据容器的类型自动填充数据，方便使用

## passwordMaker

把 passwordMaker/PasswordMaker.sample 复制为 passwordMaker/PasswordMaker.go

修改其中的 key 和 iv 的内容，长度至少32字节

执行 go run passwordMaker/*.go '你的密码' 生成加密后的密码


## tests

### 把 tests/dbInit.go.sample 复制为 tests/dbInit.go

修改其中的 key 和 iv 的内容，长度至少32字节，保持与passwordMaker 中一致

### 把 tests/db.json.sample 复制为 tests/db.json

修改你本地数据库连接信息

使用 passwordMaker 创建加密后的密码写入 db.json 中的 password

进入 tests 目录运行 go test



## 项目中的配置

### 把 /dbInit.go.sample 复制到你项目中命名为 /dbInit.go

修改其中的 key 和 iv 的内容，长度至少32字节，保持与passwordMaker 中一致

也可以以其他方式只要在 init 函数中调用 db.SetEncryptKeys 设置匹配的 key和iv即可

### 把 /db.json.sample 复制到你项目中命名为 /db.json

修改你本地数据库连接信息

使用 passwordMaker 创建加密后的密码写入 db.json 中的 password
