ALTER TABLE `expt_item_result_run_log`
    ADD COLUMN `item_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=旧数据/无版本概念; 真值源 expt_item_ref' AFTER `item_id`;

ALTER TABLE `expt_item_result_run_log`
    ADD COLUMN `quota_reservation_state` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '中心调度额度预占投影: 0=none, 1=reserved; Redis reservation 是账本真值, 本列仅供调度算准并发占用' AFTER `result_state`;

-- 刻意不为本表新增索引。
--
-- 所有 dispatch 查询（ClaimQuotaReserved / ResetQuotaReserved / StartReservedItem /
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
-- caveat：若未来 reconcile 实现成「跨 run 扫超时 reservation」（不带 expt_run_id），
-- 届时按那个查询的实际形状再评估，不要现在预建。
