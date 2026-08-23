# BENZHI 评测说明：task182-dftimeline

## 项目

数字取证时间线冲突解析服务。纯后端 Go 服务，业务闭环为"登记证据源时钟模型与工件 -> 区间归一化 -> 偏序约束 -> 时间线构建与最小冲突环检测 -> 裁决 -> 发布冻结报告"。

## 构建与测试

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
```

## 自检契约（--smoke-test）

`go run ./cmd/dftimeline --smoke-test` 不启动长驻服务，而是：

1. 创建两个证据源并提交时钟校正（laptop 偏快 30 秒）；
2. 登记"下载"与"打开"工件，换算后"下载先于打开"约束验证为 `satisfied`；
3. 构建时间线 -> `reviewable`，发布报告 v1（绑定 2 个时钟模型）；
4. **关闭并重开数据库**，验证时间线/工件/报告全部恢复；
5. 加入伪造创建时间，形成三工件约束环，时间线进入 `conflicted`，返回 3 个工件的最小冲突核心；
6. 更新源校准后重建同名时间线并发布，旧时间线 `superseded`，旧报告保持不可变。

任何断言失败以非零退出码结束；全部通过打印 `SELFCHECK OK` 并退出 0。

## Docker 双架构

- `benzhi.Dockerfile` 支持 `--platform` 交叉构建（构建阶段使用 golang:1.26.3-bookworm，运行阶段 alpine:3.20）。
- 构建：`bash build_benzhi_docker.sh <镜像名> <平台>`（默认 linux/amd64）。
- 运行验证：`docker run --rm <镜像名> --smoke-test`（ENTRYPOINT+CMD 已固定，**不要**追加 `/app/dftimeline` 路径参数，否则 flag 解析会把位置参数吞掉、服务进入长驻）。

## 主要 API

- `POST /api/sources`、`POST /api/sources/{id}/calibrations`
- `POST /api/artifacts`（内容哈希幂等，同哈希不同内容返回 409）
- `POST /api/constraints`、`POST /api/constraints/{id}/verify`
- `POST /api/timelines`、`POST /api/timelines/{id}/build`（可断点续传）
- `GET /api/timelines/{id}/conflicts`（最小冲突环 + 归因）
- `POST /api/timelines/{id}/adjudications`、`POST /api/timelines/{id}/publish`
- `GET /api/reports/{id}`（冻结快照）

完整列表见 README.md。
