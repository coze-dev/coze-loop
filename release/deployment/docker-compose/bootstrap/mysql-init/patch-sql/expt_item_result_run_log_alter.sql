ALTER TABLE `expt_item_result_run_log`
    ADD COLUMN `item_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=旧数据/无版本概念; 真值源 expt_item_ref' AFTER `item_id`;

ALTER TABLE `expt_item_result_run_log`
    ADD COLUMN `quota_reservation_state` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '中心调度额度预占投影: 0=none, 1=reserved; Redis reservation 是账本真值, 本列仅供调度算准并发占用' AFTER `result_state`;

-- 中心调度派发查询索引：按 (run, status, 预占态) 定位「可授予候选」与「已预占占用」两类 item。
-- 末位带 id 是为了让 keyset 分页与稳定排序都能走索引，避免 filesort。
ALTER TABLE `expt_item_result_run_log`
    ADD INDEX `idx_expt_run_dispatch` (`space_id`, `expt_id`, `expt_run_id`, `status`, `quota_reservation_state`, `id`);
