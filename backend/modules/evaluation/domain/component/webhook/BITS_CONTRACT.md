# BITs Callback Contract

CozeLoop 评测实验状态变更 → BITs 节点回调契约。BITs 节点 UI 侧按本文实现「实验结束回调」开关与下游节点串接;CozeLoop 侧按同一契约投递。

## 触发条件

CozeLoop dispatcher 满足以下**所有**条件时,向 BITs 平台投递一次 `bits_callback` delivery:

1. `WebhookGlobalConf.Enable = true` 且 `space_id` 不在 `DisabledSpaces` 列表内。
2. `WebhookGlobalConf.BitsCallbackURLTemplate` 非空(平台侧已开启 BITs 集成)。
3. 实验 `SourceType = SourceType_Workflow (3)`(来自 BITs / 工作流触发的实验)。
4. 事件类型为**终态**:`WebhookEventSucceeded` / `WebhookEventFailed` / `WebhookEventTerminated`。**`WebhookEventStarted`(processing)不触发**。

用户配置的 `WebhookNotificationConf` 与 BITs 回调**互相独立**:用户 URL 只在用户 filter 命中时投递,BITs 回调走独立的终态判定。

## URL 模板

平台配置示例:

```
webhook_global.bits_callback_url_template =
  https://bits.bytedance.net/v1/callback?workflow_id={source_id}&experiment_id={experiment_id}&space_id={space_id}
```

支持占位符:

| 占位符 | 来源 |
| --- | --- |
| `{source_id}` / `{workflow_id}` | `Experiment.SourceID`(BITs 工作流 ID) |
| `{experiment_id}` / `{expt_id}` | `Experiment.ID` |
| `{space_id}` / `{workspace_id}` | `Experiment.SpaceID` |

未配置模板 → dispatcher 跳过 bits_callback 注入(用户 webhook 分支不受影响)。

## 请求载荷 (Payload)

BITs 节点收到 `POST {bits_callback_url}`,请求体是 `WebhookPayload` JSON,与用户 webhook 一致:

```json
{
  "delivery_id": "<uuid>",
  "experiment_id": 123,
  "space_id": 456,
  "event_type": "succeeded",
  "status": "succeeded",
  "source_id": "wf_123",
  "result_url": "https://cozeloop.example.com/experiment/123",
  "timestamp_ms": 1730000000000
}
```

## 签名头

- `X-CozeLoop-Signature`: `hex(hmac_sha256(space_secret, timestamp + "\n" + body))`
- `X-Fornax-Signature`: 与 `X-CozeLoop-Signature` 相同(向后兼容旧 fornax 客户端)
- `X-CozeLoop-Timestamp`: 与签名 timestamp 一致(毫秒)

密钥来源(优先级):
1. `WebhookGlobalConf.SpaceSecrets[space_id]`
2. `WebhookGlobalConf.Secret`(fallback)
3. 二者皆空 → Signature 头置空,BITs 侧按需自行验签

## 幂等

BITs 侧**必须**按 `delivery_id` 去重。CozeLoop Sender 支持 MQ redeliver 场景下重复投递同一 `delivery_id`。

## 重试

CozeLoop 侧:MessageTTL 2h,delayLevel `[1min, 5min, 30min]`,MaxRetries 3。BITs 侧建议返回 5xx 触发重试,4xx / 2xx 视为终态。

## 幂等 & 出网限制

- Sender 阻止投递到 RFC1918 (10/8, 172.16/12, 192.168/16)、loopback (127/8)、link-local (169.254/16) 网段的 URL。BITs 侧地址必须是外网可达的公网地址(即使 BITs 在同 IDC,也需通过公网入口)。

## 未启用场景

以下情况 CozeLoop 侧**不**产生 bits_callback delivery(用户 webhook 分支不受影响):

- 实验 `SourceType != Workflow`(手动创建 / 非 BITs 来源实验)。
- 实验状态为 `pending` / `processing`(非终态)。
- `BitsCallbackURLTemplate` 未配置。
- `WebhookGlobalConf.Enable = false` 或 space 在 `DisabledSpaces`。

## 相关代码 anchor

- `dispatcher.go:buildBitsCallbackURL` — 注入判定
- `webhook_conf.go:BuildBitsCallbackURL` — URL 模板渲染
- `sender.go` — HMAC 签名 + CIDR 拦截
