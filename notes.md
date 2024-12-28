# Go 交易系统开发记录

## 问题记录
1. docker compose 启动报错，端口访问权限问题。
   1. [关闭winnat](https://stackoverflow.com/questions/65272764/ports-are-not-available-listen-tcp-0-0-0-0-50070-bind-an-attempt-was-made-to)
2. 关闭 winnat 之后, ubuntu 无法访问网络了, 目前连 baidu 使用 curl 都失败
   1. wsl 没有镜像到宿主机，[配置.wslconfig文件](https://github.com/microsoft/WSL/issues/10753)

## 项目启动
1. docker compose 启动这些中间件，然后本地启动服务


## internal 目录
1. models: 这里的结构除了 base 等公有字段, 都没有 gorm tag, 可能不是用来作为数据库操作的, 而是用于作为json数据传输对象(因为还定义了字段为空时省略)? 
2. modules
   1. matching: 订单匹配
   2. quote: 报价模块, 提供当前市场的价格信息，包括买卖报价、交易量等数据，以帮助交易者了解市场状况，并做出买卖决策。
   3. settlement:  结算模块, 负责交易完成后的清算和结算工作，即确保交易双方在成交后按照规则履行资金和资产的转移。
3. persistence: 用于存放与数据持久化相关的代码，比如数据库连接、ORM（对象关系映射）模型等。
   1. 定义了与所有 repo 交互的接口
   2. /gorm/entities: 定义了所有数据库表结构
   3. 其余: 实现接口的结构体和方法实现, 还有测试
   4. 但是此外, assets_repo.go 的 transfer() 原有逻辑存在一定的问题
4. services: 应用程序的服务层代码，负责处理业务逻辑和与其他服务的交互。服务层调用数据层（如 persistence）的代码，并提供 API 接口供上层调用。但是目前还是空目录，后续可以完成这里。 