# h2mux 单流接收窗口

本 fork 支持在每个代理节点的 `smux` 配置中设置 `h2mux-stream-window`，单位为字节。未设置或为 `0` 时保留默认 4 MiB；16 MiB 使用 `16777216`，32 MiB 使用 `33554432`。

```yaml
smux:
  enabled: true
  protocol: h2mux
  h2mux-stream-window: 33554432
```

参数只适用于 `h2mux`。负值、超过 `2147483647` 的值及其他复用协议误用该参数都会报错。新窗口需要 Go 1.24+；旧 Go 构建仍可使用默认窗口，但不会静默接受新窗口参数。

## 实现

- `adapter/outbound/singmux.go` 接收节点配置并传给 `sing-mux`。
- `third_party/sing-mux` 保存基于上游 `v0.3.12` 的必要定制源码和原有许可证，`go.mod` 使用相对路径 `replace`，无需 `/tmp` 补丁或另一个远端仓库。
- 通过标准库 `http.HTTP2Config.MaxReceiveBufferPerStream` 和 `http2.ConfigureTransports` 设置单流窗口；保留原有连接池、底层代理连接回调、帧大小及空闲检测。
- 接收窗口在新会话创建时通过标准 HTTP/2 SETTINGS 通告。已有会话不会因为配置文件写入而自动扩窗。

下载方向需要在客户端设置，例如 gw。服务端会处理标准窗口通告，不要求同步更新。32 MiB 按流计算，应结合活跃流数量观察内存。

## 构建与检查

```bash
make h2window-test
make h2window-build
```

候选二进制默认写入 `/tmp/mihomo-h2window`，构建使用 `GOAMD64=v3`、`CGO_ENABLED=0` 和 `with_gvisor`。也可以通过 `H2WINDOW_BIN` 指定其他临时路径。

测试覆盖配置解析及错误参数、实际 4/16/32 MiB SETTINGS、连接级额度不变、同一会话上的多流复用。

本改动基于 fork 当前的 `Meta` 分支，未合入 `Alpha` 的其他上游变更。
