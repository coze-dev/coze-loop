ALTER TABLE `expt_item_result_run_log`
    ADD COLUMN `item_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=旧数据/无版本概念; 真值源 expt_item_ref' AFTER `item_id`;

ALTER TABLE `expt_item_result_run_log`
    ADD COLUMN `retry_times` int unsigned NOT NULL DEFAULT '0' COMMENT '本轮实验运行中该 item 已被系统自动重试的次数; 0=未重试; 仅内部调度降权用, 不透出' AFTER `result_state`;

ALTER TABLE `expt_item_result_run_log`
    ADD KEY `idx_expt_run_retry_pick` (`space_id`, `expt_id`, `expt_run_id`, `status`, `retry_times`, `id`);
