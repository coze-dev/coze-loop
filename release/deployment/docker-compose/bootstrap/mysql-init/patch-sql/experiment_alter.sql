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

ALTER TABLE `experiment` ADD COLUMN `offline_expt_analysis_status` int unsigned NOT NULL DEFAULT '0' COMMENT '离线实验分析状态：0-未开始，1-进行中，2-成功，3-失败，4-已被取代(superseded)';

ALTER TABLE `experiment` ADD COLUMN `notification_conf` blob COMMENT '通知配置，json格式存储webhook/飞书通知配置';

ALTER TABLE `experiment` ADD COLUMN `experiment_group_key` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '实验分组key，默认实验ID' AFTER `name`;

ALTER TABLE `experiment`
    ADD INDEX `idx_experiment_group_key_deleted_at` (`experiment_group_key`, `deleted_at`);

ALTER TABLE `experiment`
    ADD COLUMN `eval_set_source_type` int unsigned NOT NULL DEFAULT '1' COMMENT '评测集来源模式: 1=SingleSet(老,单评测集) / 2=MultiSetConfig(新,多评测集+配置,权威源 eval_conf)' AFTER `eval_set_id`;


ALTER TABLE `experiment`
    ADD COLUMN `eval_set_space_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '评测集来源空间(跨空间共享,0=同空间)' AFTER `eval_set_source_type`;

ALTER TABLE `experiment`
    ADD COLUMN `target_space_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '评测对象来源空间(跨空间共享,0=同空间)' AFTER `eval_set_space_id`;

ALTER TABLE `experiment`
    ADD COLUMN `eval_set_access_level` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '发起冻结的评测集访问级别(execute/readable/空=同空间)' AFTER `target_space_id`;

ALTER TABLE `experiment`
    ADD COLUMN `priority_level` int unsigned NOT NULL DEFAULT '1' COMMENT '实验调度优先级，1-99，数值越大越优先' AFTER `notification_conf`;

ALTER TABLE `experiment`
    ADD COLUMN `scheduler_mode` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT 'legacy' COMMENT '调度模式：legacy(旧per-experiment链路)/enforce(中心调度)' AFTER `priority_level`;

-- 中心调度所有权与 Priority 排序边界。线上与各 PPE 泳道共用同一个库，缺此列则泳道调度器会扫出
-- 线上实验并为其派发 item（结果写回共享库、线上侧无感知）。legacy 历史行保持空串。
ALTER TABLE `experiment`
    ADD COLUMN `scheduler_scope` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '中心调度所有权与Priority排序边界; legacy为空' AFTER `scheduler_mode`;

-- 中心调度主扫描索引：scheduler_mode + scheduler_scope + status 定位候选，priority_level DESC/created_at/id 提供稳定排序
-- 注意：降序索引需 MySQL 8.0+；低版本会静默忽略 DESC 退化为升序，上线前须确认实例版本并 EXPLAIN 验证
ALTER TABLE `experiment`
    ADD INDEX `idx_scheduler_queue` (`scheduler_mode`, `scheduler_scope`, `status`, `deleted_at`, `priority_level` DESC, `created_at`, `id`);
