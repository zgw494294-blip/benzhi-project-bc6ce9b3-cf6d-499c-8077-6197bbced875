# 频率变更启用审查台

本项目是面向无线电台站技术人员的频率变更安全审查 HTTP 服务。它把拟变更发射参数、受保护对象、确定性干扰核验、冲突整改、独立复核、基线冻结和负责人审批串成一个可审计流程，审批通过后签发带递增序号和内容摘要的不可变启用许可。

## 业务状态

案件从 `draft` 开始。提交时封存参数基线并执行 `frequency-separation` 与 `field-strength` 两类规则；无冲突时进入 `reviewed`，存在冲突时进入 `remediating`。每项整改必须提交调整参数和证据摘要，并由不同主体的 `reviewer` 接受。最新全量核验通过后可进入 `frozen`，最后由 `leader` 批准为 `approved` 或驳回为 `rejected`。许可和已封存参数由 SQLite 触发器保护，不能更新或删除。

所有写请求都要求 `Idempotency-Key`、`X-Actor` 和 `X-Role` 请求头。角色值为 `planner`、`reviewer` 或 `leader`。除创建外的写请求还必须在 JSON 中提供当前 `expectedRevision`，修订不一致会返回 `409 Conflict`。幂等记录同时绑定操作、案件、操作者和规范化请求摘要；只有完全相同的重试才会重放首次响应，误用同一键会返回首次请求元数据而不会泄露请求正文。

案件生效窗口统一规范化为 UTC，最长允许 366 天。创建、编辑和送审都会按半开区间 `[effectiveFrom, effectiveUntil)` 检查同一台站已冻结或已批准案件的窗口占用，边界相接可以通过。

## 构建和运行

下载依赖并构建：

```bash
go mod download
go build ./cmd/server
```

默认仅监听高位回环地址 `127.0.0.1:19081`，数据保存到 `frequency-review.db`：

```bash
go run ./cmd/server
```

可以显式指定回环地址和数据库文件：

```bash
go run ./cmd/server -addr=127.0.0.1:19121 -db=review.db
```

未显式传递 `-addr` 时，也可通过 `PORT` 传入端口号，服务会绑定 `127.0.0.1:<PORT>`。通配地址和低于 1024 的端口会被拒绝。

## API

版本化 API 根路径为 `/api/v1`。主要入口包括：

- `POST /api/v1/change-cases`：创建案件及初始发射参数。
- `PUT /api/v1/change-cases/{caseID}`：在草拟阶段更新案件和参数。
- `POST /api/v1/change-cases/{caseID}/targets`：添加保护对象。
- `POST /api/v1/change-cases/{caseID}/targets/batch`：原子新增、更新和删除一批保护对象。
- `GET|POST /api/v1/change-cases/{caseID}/checks/preview`：只读预览送审核验和 `previewDigest`。
- `POST /api/v1/change-cases/{caseID}/submit`：封存送审基线并全量核验。
- `GET /api/v1/change-cases/{caseID}/checks/trace`：按基线、目标和规则分页查询可复现核验轨迹。
- `POST /api/v1/change-cases/{caseID}/conflicts/{conflictID}/resolution`：提交整改参数和证据。
- `POST /api/v1/change-cases/{caseID}/conflicts/batch-resolution`：使用一组统一参数原子整改多个冲突。
- `POST /api/v1/change-cases/{caseID}/conflicts/{conflictID}/review`：独立接受或退回整改。
- `POST /api/v1/change-cases/{caseID}/conflicts/batch-review`：独立复核员原子提交一批逐项结论。
- `GET /api/v1/change-cases/{caseID}/freeze/readiness`：只读查询冻结统计和结构化阻断项。
- `POST /api/v1/change-cases/{caseID}/freeze`：再次全量核验并冻结基线。
- `POST /api/v1/change-cases/{caseID}/decision`：负责人批准或驳回。
- `GET /api/v1/change-cases/{caseID}`：查询完整案件视图。
- `GET /api/v1/change-cases/{caseID}/timeline`：查询连续摘要审计时间线。
- `GET /api/v1/change-cases/{caseID}/permit/verify`：重算许可和审计摘要。

JSON 请求体上限为 1 MiB，未知字段和非 `application/json` 写请求会被拒绝。`GET /readyz` 是有界数据库就绪探针。

## 自检和测试

自检会建立内存 SQLite 数据库，实际监听指定地址，通过 HTTP 完成案件创建、保护对象登记、送审核验、冻结、批准、许可签发及摘要验证，然后主动关闭服务：

```bash
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
```

运行全部回归测试：

```bash
go test ./...
```

测试覆盖确定性规则、参数校验、审计防篡改、幂等重放、修订并发保护、冲突整改、独立复核、许可签发和批准后禁止修改。
