CREATE TABLE IF NOT EXISTS `expt_item_result`
(
    `id`          bigint unsigned NOT NULL DEFAULT '0' COMMENT 'id',
    `space_id`    bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 id',
    `expt_id`     bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验 id',
    `expt_run_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验运行 id',
    `item_id`     bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item_id',
    `item_idx`    int unsigned             DEFAULT NULL COMMENT 'item 序号',
    `status`      int unsigned    NOT NULL DEFAULT '0' COMMENT '状态',
    `err_msg`     blob COMMENT '错误信息',
    `created_at`  timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`  timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`  timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    `log_id`      varchar(128)    NOT NULL DEFAULT '' COMMENT '日志 id',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_expt_item_idx` (`space_id`, `expt_id`, `item_id`),
    KEY `idx_expt_status` (`space_id`, `expt_id`, `status`),
    KEY `idx_expt_item_turn_idx` (`space_id`, `expt_id`, `item_idx`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='expt_item_result';