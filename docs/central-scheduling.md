# 中心调度运维入口

本服务通过 `EvalSysOperationService` 暴露四个内部入口：

- `CentralScheduleTick`：执行一次 scope 级派发 tick，返回是否为 leader。
- `GetRunScheduleStatus`：回读 `expt_run_schedule` 投影。
- `ReconcileRun`：增量对账或单 run 全量重建。
- `RebuildScope`：scope 级全量重建。

tick 由进程内独立分钟级 ticker 驱动，不依赖旧 per-experiment MQ 自循环；
多实例通过 Redis scope leader lease 收敛为单实例派发。
