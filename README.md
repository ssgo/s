# ssGo框架基础支持

## Configs

使用json格式管理配置，并且支持用环境变量来覆盖任何配置

传入一个结构体来读取配置方便使用，虽然也支持 map[string]interface{} 但并不推荐使用

## Encoder

支持aes算法的加解密

## passwordMaker

把 passwordMaker/PasswordMaker.sample 复制为 passwordMaker/PasswordMaker.go

修改其中的 key 和 iv 的内容，长度至少32字节

执行 go run passwordMaker/*.go '你的密码' 生成加密后的密码

