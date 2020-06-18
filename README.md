# ethereum-oracle-service
ethereum oracle service

## 1、编译
```
go build
```
编译完成后查看帮助信息

```
./oracle-service -h
oracle_service version: 1.0.0
Usage: oracle_service [-h help] [-v version] [-c config path] [-l log path]
```
## 2、配置

配置信息如下：

```
# 合约地址
OracleContractAddress = ""
# 网络ws地址
NetworkWS = "ws://"
# 调用合约的私钥
PrivateKey = ""
```

## 3、运行
```
./oracle-service -c ./conf/app.conf -l logs/
```