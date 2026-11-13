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

本地构建和 CI 打包统一使用 `<上游版本>-h2`，当前为 `v1.19.34-h2`；二进制版本和压缩包文件名均使用此版本。同步上游后更新 `Makefile` 中的 `UPSTREAM_VERSION`，也可以在构建时通过 `UPSTREAM_VERSION` 覆盖。

测试覆盖配置解析及错误参数、实际 4/16/32 MiB SETTINGS、连接级额度不变、同一会话上的多流复用。

本改动基于 fork 当前的 `Meta` 分支，未合入 `Alpha` 的其他上游变更。

## 同步上游与发布

更新前先提交需要保留的本地修改。首次配置上游：

```bash
git remote add upstream https://github.com/MetaCubeX/mihomo.git
```

在 `Meta` 分支按正式版本合并，以下版本号替换为目标版本：

```bash
git switch Meta
git pull --ff-only origin Meta
git fetch --no-tags upstream refs/tags/v1.19.34
git merge --no-edit FETCH_HEAD
```

直接合并 `FETCH_HEAD`，避免本地或 fork 的同名标签与上游标签指向不同提交。冲突需要先解决；取消合并可执行 `git merge --abort`。如果上游更新了 `sing-mux`，还需同步 `third_party/sing-mux` 的依赖源码并保留定制补丁。

更新 `Makefile` 中的 `UPSTREAM_VERSION` 及本文版本说明，执行 `make h2window-test`、`make h2window-build`，检查 `/tmp/mihomo-h2window -v`，然后提交修改。发布时使用独立的 `-h2` 标签：

```bash
git push origin Meta
git tag -a v1.19.34-h2 -m 'Release v1.19.34-h2'
git push origin v1.19.34-h2
```

`Release h2` 工作流要求标签与 `make version` 一致，测试通过后构建 Linux x86_64 通用版（v1）、x86_64-v3 和 ARM64，自动上传 `.gz`、`version.txt` 和 `checksums.txt` 到 GitHub Release。发布成功后可从 Actions 页面和 Release 资产列表核对结果。

原上游的 Build、Test、CMFA 工作流仅在 `MetaCubeX/mihomo` 运行；本 fork 的发布统一由 `Release h2` 处理。
