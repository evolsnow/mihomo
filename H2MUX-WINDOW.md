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

以下流程已于 2026-11-13 在本仓库完成一次实际发布。按小节依次执行，任一命令失败先处理原因，再继续下一步。完成标准是 GitHub Release 有可下载且通过校验的编译产物。

### 1. 准备仓库和目标版本

固定使用 `Meta` 分支，`origin` 是 `evolsnow/mihomo`，`upstream` 是 `MetaCubeX/mihomo`。上游默认分支 `main` 不作为本 fork 的合并目标；按上游正式 release 的 tag 合并。

```bash
cd /root/mihomo
git status --short --branch
git remote -v
gh auth status
go version
```

先提交需要保留的本地修改，其他任务的草稿单独处理。临时工作可用 `git stash push -u` 暂存，合并后再 `git stash pop`；恢复时的冲突也要解决并重新测试。提交时明确列出相关文件，避免把无关草稿一起发布。

本机已配置好上游；新克隆的仓库仅在没有 `upstream` 时添加：

```bash
if ! git remote get-url upstream >/dev/null 2>&1; then
  git remote add upstream https://github.com/MetaCubeX/mihomo.git
fi
git config remote.upstream.tagOpt --no-tags
```

先查询上游最新正式版本，再明确选择本次要合入的版本：

```bash
gh api repos/MetaCubeX/mihomo/releases/latest --jq '{tag_name, published_at, html_url}'

upstream_version=v1.19.34
release_version="${upstream_version}-h2"
```

这里的 `v1.19.34` 是本次实跑示例，`v1.19.34-h2` 已发布。以后更新时替换为新的目标版本，不要重复创建旧的发布标签。后续命令使用这两个变量，在同一个 shell 中执行。

### 2. 合并正式版本，检查定制补丁

```bash
git switch Meta
git pull --ff-only origin Meta
git fetch --no-tags upstream "refs/tags/${upstream_version}"
git log --oneline HEAD..FETCH_HEAD
git merge --no-edit FETCH_HEAD
```

抓取目标 tag 后直接合并 `FETCH_HEAD`，中间不要再执行其他 `git fetch`。本地 `v1.19.34` 标签曾指向 fork 的合并提交，与上游同名标签不同，因此不要依赖本地同名标签判断上游源码，也不要通过强制覆盖标签来解决冲突。

已经合入目标版本时会显示 `Already up to date`。出现文件冲突时，用 `git status` 查看，解决后暂存对应文件并提交；取消尚未完成的合并可执行 `git merge --abort`。

合并后重点检查：

- `adapter/outbound/singmux.go` 的 `h2mux-stream-window` 配置接线及对应测试仍然存在。
- `go.mod` 中的 `replace github.com/metacubex/sing-mux => ./third_party/sing-mux` 保留。
- 如果上游升级 `sing-mux`，同步检查并更新 `third_party/sing-mux` 的源码基线，保留窗口补丁、测试和 `LOCAL-CHANGES.md`。只修改 `require` 的版本号不会更新这份本地源码。
- `.github/workflows/release-h2.yml` 保留，原上游 Build、Test、CMFA 工作流的仓库限制仍然存在。

原上游 Build 的手动发布步骤含有将 `Alpha` 强制推送到 `Meta` 的逻辑。该工作流在本 fork 被限制为跳过，定制版统一通过 `Release h2` 发布。

### 3. 更新版本号，完成本地验证

将 `Makefile` 的 `UPSTREAM_VERSION` 改成目标上游版本，并同步本文“构建与检查”中的当前版本说明。默认规则保持 `VERSION ?= $(UPSTREAM_VERSION)-h2`。

```bash
make -s version
test "$(make -s version)" = "$release_version"
```

发布标签必须与 `make version` 完全一致，CI 会检查这一点。仅在本地通过 `UPSTREAM_VERSION=...` 覆盖构建参数不会修改 GitHub 上的默认版本，正式发布前必须提交 `Makefile` 的版本更新。

本机已验证可用的模块缓存是 `/tmp/mihomo-custom/go-mod`；临时目录被清理后，Go 会重新下载依赖。其他机器也可以使用自己的正常缓存。

```bash
export GOMODCACHE=/tmp/mihomo-custom/go-mod

make h2window-test
CGO_ENABLED=0 SKIP_INTEROP_TEST=1 SKIP_CONCURRENT_TEST=1 \
  go test -p 2 -tags with_gvisor -count=1 -timeout 10m ./...
make h2window-build
/tmp/mihomo-h2window -v
```

`h2window-test` 同时覆盖本地替换模块和配置解析，不能只用主模块的 `go test ./...` 代替。整仓测试的两个跳过项与 CI 一致，跳过外部实现互操作测试和并发压力用例。候选二进制应输出本次的 `release_version` 和 `with_gvisor`。

需要在本地提前验证全部发布架构时执行：

```bash
GOFLAGS=-p=2 make BINDIR="/tmp/mihomo-release-${release_version}" \
  linux-amd64-v1.gz linux-amd64-v3.gz linux-arm64.gz
```

CI 使用 MetaCubeX 提供的 Go 工具链，当前系列为 `1.26`，以 `.github/workflows/release-h2.yml` 为准。上游提高工具链要求时，本地工具链和该工作流都需要检查。

### 4. 提交并推送发布标签

先审阅改动，再提交。以下暂存命令适用于只有版本号与文档更新的情况；如果还有补丁适配或工作流修改，将对应文件明确加入暂存区。Git 提示身份缺失时，先配置本仓库的 `user.name` 和 `user.email`。

```bash
git diff --check
git diff --stat
git add Makefile H2MUX-WINDOW.md
git diff --cached
git commit -m "chore: release ${release_version}"

git tag -a "$release_version" -m "Release ${release_version}"
git push --atomic origin Meta "refs/tags/${release_version}"
```

原子推送让分支和标签一起成功或一起失败。若提示远端分支已前进，重新抓取并协调改动后再验证，不要强推。不要移动已经公开发布的 tag。

只推送 `Meta` 不会触发发布。`Release h2` 由新推送的 `v*-h2` 标签触发，从标签对应提交构建；无需合并到 `main` 或 `Alpha`。

### 5. 等待 GitHub Actions，验证 Release

使用完整提交 SHA 查找工作流；短 SHA 可能查不到结果。推送后如果暂时返回空列表，等 GitHub 创建运行记录后重查，不必重新打标签。

```bash
release_sha="$(git rev-parse "${release_version}^{commit}")"
gh run list --repo evolsnow/mihomo --workflow release-h2.yml \
  --commit "$release_sha" --event push \
  --json databaseId,status,conclusion,url

run_id="$(gh run list --repo evolsnow/mihomo --workflow release-h2.yml \
  --commit "$release_sha" --event push --limit 1 \
  --json databaseId --jq '.[0].databaseId')"
test -n "$run_id" && test "$run_id" != null
gh run watch "$run_id" --repo evolsnow/mihomo --exit-status --interval 20
```

工作流依次完成 `verify`、三个架构的 `build`、`release`，全部成功后发布正式 Release 并设为 Latest。原上游的 Build、Test、CMFA 工作流显示 skipped 是预期行为，检查的是 `Release h2` 的结果。

发布后查看资产，并实际下载验证：

```bash
gh release view "$release_version" --repo evolsnow/mihomo \
  --json tagName,url,isDraft,isPrerelease,assets

download_dir="$(mktemp -d "/tmp/mihomo-${release_version}.XXXXXX")"
gh release download "$release_version" --repo evolsnow/mihomo --dir "$download_dir"
(cd "$download_dir" && sha256sum -c checksums.txt && gzip -t ./*.gz)

gzip -dc "$download_dir/mihomo-linux-amd64-v3-${release_version}.gz" \
  > "$download_dir/mihomo"
chmod +x "$download_dir/mihomo"
"$download_dir/mihomo" -v
```

上述运行验证适用于支持 v3 的 x86_64 主机。ARM64 主机改用 ARM64 资产；在 x86_64 主机上只能用 `go version -m` 等工具检查 ARM64 文件的构建信息，不能直接执行。

Release 应包含以下五个资产，校验和应全部匹配，二进制版本应等于标签：

| 资产 | 用途 |
| --- | --- |
| `mihomo-linux-amd64-v1-<版本>.gz` | Linux x86_64 通用版 |
| `mihomo-linux-amd64-v3-<版本>.gz` | Linux x86_64-v3 优化版 |
| `mihomo-linux-arm64-<版本>.gz` | Linux ARM64 |
| `checksums.txt` | 三个压缩包及 `version.txt` 的 SHA-256 |
| `version.txt` | 发布版本号 |

### 常见问题与恢复

- **Go 缓存损坏：** 若出现大量 `no required module provides package`，先检查模块缓存的解压目录是否为空。曾遇到 `/tmp/go-mod` 存在下载包但解压内容损坏；可使用新的 `GOMODCACHE` 重试，不要逐个 `go get` 或随意 `go mod tidy` 改动固定依赖。
- **GitHub 任务失败：** 使用 `gh run view "$run_id" --repo evolsnow/mihomo --log-failed` 查看原因。网络或运行器临时故障可用 `gh run rerun "$run_id" --repo evolsnow/mihomo --failed` 重跑；版本不匹配、编译或测试失败必须先修复原因。
- **同一上游版本需要再发修订版：** 先明确新版本命名，再同步调整 `VERSION` 和工作流的标签匹配及上游版本提取逻辑；当前流程按 `<上游版本>-h2` 工作，不能直接假设 `-h2.1` 也会触发。
- **GitHub 上传成功但本地校验失败：** 重新下载到新的临时目录并核对 Release 资产；校验通过前不把该文件作为可部署产物。

缓存损坏时可从干净目录开始，完整下载缓存存在时也可复用下载包：

```bash
export GOMODCACHE="$(mktemp -d /tmp/mihomo-release-mod.XXXXXX)"
if [ -d /tmp/go-mod/cache/download ]; then
  mkdir -p "$GOMODCACHE/cache"
  ln -s /tmp/go-mod/cache/download "$GOMODCACHE/cache/download"
fi
make h2window-test
```

### 2026-11-13 实跑记录

| 项目 | 已验证结果 |
| --- | --- |
| 上游版本 | `v1.19.34`，提交 `0455d0d28c4526937380505153e65a843d8612e8` |
| 定制版发布提交 | `7cc329bde8e9103bd318d3c23d9bdb85edb65049` |
| Release | [v1.19.34-h2](https://github.com/evolsnow/mihomo/releases/tag/v1.19.34-h2) |
| GitHub Actions | [Release h2 #43339372970](https://github.com/evolsnow/mihomo/actions/runs/43339372970)，所有任务成功 |
| 本地验证 | `h2window-test` 通过，整仓测试 60 个包通过，三个架构均编译成功 |
| 下载验证 | 五个资产齐全，SHA-256 和 gzip 检查通过，两个 x86_64 版本均成功执行 `-v`，ARM64 构建信息正确 |
| 工具链 | 本地 Go `1.26.5`；此次 GitHub 产物使用 Go `1.26.11` |
| 本机 CPU | Intel N100，4 个逻辑核心；系统报告 `x86-64-v3 (supported, searched)`，发布的 v3 二进制可正常运行 `-v` |

CPU 结论只代表这台机器在该日期暴露的能力。其他主机，尤其 VPS，需要分别确认；不确定时选择 x86_64 通用版。本流程完成源码更新和 GitHub Release 发布，线上服务部署另行执行。
