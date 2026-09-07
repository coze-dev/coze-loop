ALTER TABLE `expt_item_result_run_log`
    ADD COLUMN `item_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=旧数据/无版本概念; 真值源 expt_item_ref' AFTER `item_id`;

ALTER TABLE `expt_item_result_run_log`
    ADD COLUMN `retry_times` int unsigned NOT NULL DEFAULT '0' COMMENT '本轮实验运行中该 item 已被系统自动重试的次数; 0=未重试; 仅内部调度降权用, 不透出' AFTER `result_state`;

ALTER TABLE `expt_item_result_run_log`
    ADD COLUMN `quota_reservation_state` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '中心调度额度预占投影: 0=none, 1=reserved; Redis reservation 是账本真值, 本列仅供调度算准并发占用' AFTER `result_state`;

-- 刻意不为本表新增索引。
--
-- 【中心调度侧理由】所有 dispatch 查询（ClaimQuotaReserved / ResetQuotaReserved / StartReservedItem /
-- LoadDispatchRuntime / MGetDispatchObservations）的 WHERE 恒以
-- (space_id, expt_id, expt_run_id) 打头 —— 中心调度只扫「当前 run 的 run log」，
-- 跨实验扫描发生在 experiment 表、不在本表。该前缀已被既有索引完全覆盖：
--   uk_expt_run_item_turn(space_id,expt_id,expt_run_id,item_id) UNIQUE —— 带 item_id 的
--     精确 CAS 直接命中唯一索引定位单行，status/quota_reservation_state 只是回表判断；
--   idx_expt_run_result_state(space_id,expt_id,expt_run_id,result_state) —— 不带 item_id 的
--     LoadDispatchRuntime 走前三列前缀。
--
-- 单个 run 的 run log 实测仅 ~900 行（内场最大 914），三列前缀定位后按
-- status/quota_reservation_state 过滤是内存操作。而本表内场 7800 万行
-- （experiment 才 23 万），为几百行的内存过滤给大表加 6 列复合索引，
-- 收益接近零、代价是在线 DDL + 长期写放大。
--
-- 【让位降权侧理由（retry_times）】同结论：曾设计 idx_expt_run_retry_pick
-- (space_id, expt_id, expt_run_id, status, retry_times, id) 为 scanToSubmit 的
-- 「retry_times asc, id asc」提供索引序，但实测线上该表为高频写入热表
-- （CN 某库 99.6M 行 / 36.1GB，日均新增约 830 万行），建 6 列复合索引的构建期风险
-- （全表扫描 + 并发 DML 进 online log，写入过快可致 DDL 失败）与长期写放大，
-- 均高于 filesort 的代价 —— 而 filesort 的扫描范围只是「单实验单次运行的 Queueing 行」
-- （评测集条目量级），前置过滤仍走 uk_expt_run_item_turn 前三列。
-- 故代码侧以 RetryYieldConf.index_ready 开关兼容两种形态：默认 false 不下 ForceIndex
-- hint（由优化器自选，退化 filesort，排序语义不变）；将来若在低峰期补建索引，
-- 翻 index_ready=true 即取最优执行计划，无需改代码。
--
-- caveat：若未来 reconcile 实现成「跨 run 扫超时 reservation」（不带 expt_run_id），
-- 届时按那个查询的实际形状再评估，不要现在预建。
