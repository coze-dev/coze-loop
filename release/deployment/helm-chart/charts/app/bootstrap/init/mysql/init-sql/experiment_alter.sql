ALTER TABLE `experiment`
    ADD COLUMN `expt_template_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验模板 id' AFTER `eval_set_id`;

ALTER TABLE `experiment`
    ADD INDEX `idx_space_expt_template_id_delete_at` (`space_id`, `expt_template_id`, `deleted_at`);

ALTER TABLE `experiment`
    ADD COLUMN `trigger_type` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT 'manual' COMMENT '实验触发方式：manual/openapi/schedule' AFTER `max_alive_time`;

ALTER TABLE `experiment`
    ADD INDEX `idx_space_trigger_type_delete_at` (`space_id`, `trigger_type`, `deleted_at`);

ALTER TABLE `experiment` ADD COLUMN `visibility` int unsigned NOT NULL DEFAULT '0' COMMENT '可见性，默认0-可见，1-隐藏';

ALTER TABLE `experiment` ADD COLUMN `thread_id` varchar(255) DEFAULT NULL COMMENT '智能生成会话ID';

ALTER TABLE `experiment` ADD COLUMN `trial_run_item_count` bigint unsigned DEFAULT NULL COMMENT '试运行行数';
