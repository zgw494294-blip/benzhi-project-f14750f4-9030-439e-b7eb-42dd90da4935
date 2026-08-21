# 舞台吊挂演出就绪验证台

舞台吊挂演出就绪验证台是面向剧场技术负责人的本地 Web 应用。它把演出前的设备登记、动作基线、干运行实测、安全复核、整改重测和就绪证书串成一条可审计的状态流程。系统以 JSON Lines 事件日志为事实来源，使用 `previousHash`、`checksum` 和连续版本保证事件链完整，并能从日志重放出会话投影。

## 构建与运行

项目使用 Go 标准库，不需要 Node 构建链。

```text
go test ./...
go run ./cmd/stageready -addr=127.0.0.1:19081
```

启动后访问 `http://127.0.0.1:19081/sessions`。默认数据写入 `data/stageready`，也可以用 `-data-dir` 指定目录。监听地址必须是回环地址，默认端口为 `19081`；未显式传入 `-addr` 时，可用 `PORT=19082` 绑定 `127.0.0.1:19082`。

## 自检与测试

完整自检会在临时数据目录启动真实 HTTP 服务，依次调用创建会话、配置设备和动作、确认方案、启动干运行、录入实测、完成复核、签发证书及查询证书端点，验证状态为 `Certified` 且摘要有效后自动关闭并清理临时目录。

```text
go run ./cmd/stageready -selfcheck -addr=127.0.0.1:19081
```

标准回归命令：

```text
go test ./...
go run ./cmd/stageready -selfcheck -addr=127.0.0.1:19081
```

主要 JSON 端点包括 `GET /api/sessions`、`POST /api/sessions`、`GET /api/sessions/{id}`，以及设备、动作、方案确认、干运行、实测、复核、整改和证书相关的 `POST` 路由。所有修改请求都带 `expectedVersion` 与 `idempotencyKey`，重复提交会返回原提交结果。

## 扩展工作流

- Draft 基线支持设备和动作修订、安全删除与动作原子重排；冻结后统一拒绝修改。设备载荷或急停要求变化会重新校验全部关联动作。
- `POST /api/sessions/{id}/configuration/preflight` 提供批量配置预检，`POST /api/sessions/{id}/configuration/batch` 以全有或全无方式确认设备和动作。
- `POST /api/sessions/{id}/attempts/batch` 从当前 Pending 动作开始录入连续实测，并在完成最后一个待办时自动进入 Review。
- Correction 状态通过 `PUT /api/sessions/{id}/corrections/{cueID}` 维护与失败实测绑定的整改任务；全部任务闭合后才能提交整改和重测。
- `GET /api/sessions` 支持 `status`、`venue`、`technicalDirector`、`performanceFrom`、`performanceTo`、`q`、`sort`、`order`、`page` 和 `pageSize` 组合查询，同时返回当前筛选范围的风险摘要。
- `GET /api/sessions/{id}/certificates/{certificateID}/verification` 只读验证证书摘要、签发事件载荷和签发前事件链，也可用 `digest`、`eventHeadHash` 与 `sessionVersion` 查询参数比对外部持有值。
