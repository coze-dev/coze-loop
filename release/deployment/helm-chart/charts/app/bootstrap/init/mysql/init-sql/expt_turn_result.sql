CREATE TABLE IF NOT EXISTS `expt_turn_result`
(
    `id`               bigint unsigned NOT NULL DEFAULT '0' COMMENT 'id',
    `space_id`         bigint unsigned NOT NULL COMMENT '空间 id',
    `expt_id`          bigint unsigned NOT NULL COMMENT '实验 id',
    `expt_run_id`      bigint unsigned NOT NULL COMMENT '实验运行 id',
    `item_id`          bigint unsigned NOT NULL COMMENT 'item_id',
    `turn_id`          bigint unsigned NOT NULL DEFAULT '0' COMMENT 'turn_id',
    `turn_idx`         int unsigned             DEFAULT NULL COMMENT 'turn 序号',
    `status`           int unsigned    NOT NULL DEFAULT '0' COMMENT '状态',
    `trace_id`         bigint unsigned NOT NULL DEFAULT '0' COMMENT 'trace_id',
    `log_id`           varchar(128)    NOT NULL DEFAULT '' COMMENT '日志 id',
    `target_result_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'target_result_id',
    `err_msg`          blob COMMENT '错误信息',
    `created_at`       timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`       timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_expt_item_turn` (`space_id`, `expt_id`, `item_id`, `turn_id`),
    KEY `idx_expt_status` (`space_id`, `expt_id`, `status`),
    KEY `idx_expt_item_turn_idx` (`space_id`, `expt_id`, `item_id`, `turn_idx`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='expt_turn_result';