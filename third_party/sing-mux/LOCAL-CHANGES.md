# sing-mux 定制源码

来源：`github.com/metacubex/sing-mux v0.3.12`。

原始模块校验值：`h1:80jrJsGgRW7ZHpJCRpwJhG3yzT5muEHIWjaOsDJZ2wE=`。

保留上游源码、测试和许可证。定制部分包括可配置的 h2mux 单流接收窗口、Go 1.24+ 公开 API 接线、旧 Go 的明确错误处理，以及窗口和连接复用测试。

`x/net` 最低版本对齐到 Mihomo 使用的 `v0.35.0`，以支持公开 HTTP/2 配置接线。
