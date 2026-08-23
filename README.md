# 数字取证时间线冲突解析服务 (task182-dftimeline)

面向数字取证分析师的时间线构建与冲突解析服务。分析师登记证据源（文件系统、日志、设备、网络）的时钟模型和带时间戳的工件，声明"下载先于打开"之类的偏序关系；系统把本地时间戳换算为不确定区间，构建偏序图，找出可排序事件、重叠事件与不可满足的约束环，并输出最小冲突工件集与归因（时钟偏差 / 元数据篡改 / 证据不足）。分析师确认时钟校正或标记可疑工件后，发布绑定时钟模型版本的冻结报告。

## 业务闭环

1. 登记证据源并提交时钟校正（时区偏移 + 偏差 + 不确定度）。
2. 登记工件（RFC3339 时间戳必须带时区），系统换算为不确定区间。
3. 声明偏序约束（A 先于 B），区间验证得到 satisfied / violated / insufficient。
4. 构建时间线：偏序图 + 最小环检测，输出冲突核心与归因。
5. 人工裁决（采纳 / 可疑 / 隔离源 / 校正时钟）后发布冻结报告；更新校准只产生替代报告。

## 核心状态机

- 证据源：`pending_calibration → calibrated / unknown_offset / isolated`，`isolated → pending_calibration`。
- 工件：`pending → normalized → conflict / suspicious / adopted`。
- 约束：`pending → satisfied / violated / insufficient`。
- 时间线：`building → reviewable / conflicted → published → superseded`。
- 报告：`draft → frozen`（发布后不可变）。

## 标准命令

```bash
# 构建 / 静态检查 / 测试
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...

# 端到端自检（创建数据 -> 核心闭环 -> 关闭重开数据库验证恢复，exit 0）
go run ./cmd/dftimeline --smoke-test

# 启动服务
go run ./cmd/dftimeline --addr :8090 --db dftimeline.db
```

## API 入口（前缀 /api）

| 能力 | 入口 |
| --- | --- |
| 证据源 | POST/GET `/api/sources`、GET `/api/sources/{id}`、`/api/sources/{id}/calibrations`、`/mark-unknown`、`/isolate`、`/reactivate`、`/artifacts` |
| 工件 | POST/GET `/api/artifacts`、GET `/api/artifacts/{id}`、`/suspicious`、`/adopt` |
| 约束 | POST/GET `/api/constraints`、GET `/api/constraints/{id}`、`/verify` |
| 时间线 | POST/GET `/api/timelines`、`/build`、`/conflicts`、`/conflicts/{cid}`、`/adjudications`、`/publish`、`/reports` |
| 报告 | GET `/api/reports`、GET `/api/reports/{id}` |
| 运维 | GET `/api/health`、GET `/api/stats`、POST `/api/selfcheck` |

## 持久化

SQLite（纯 Go 驱动 `modernc.org/sqlite`）：`evidence_sources`、`clock_calibrations`、`artifacts`（内容哈希唯一）、`constraints`、`timelines`（构建游标持久化）、`conflict_cores`、`adjudications`、`reports`。重启后从解析游标恢复构建；同一工件哈希不可重复写入。

## 错误边界

拒绝无时区时间戳、区间反转、同哈希不同内容、跨案件关联、发布报告直接重排；约束环必须返回最小环证据（受限 BFS，迭代有上限）。
