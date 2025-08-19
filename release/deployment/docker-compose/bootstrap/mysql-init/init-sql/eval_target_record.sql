CREATE TABLE IF NOT EXISTS `eval_target_record`
(
    `id`                bigint unsigned NOT NULL COMMENT 'id',
    `space_id`          bigint unsigned NOT NULL COMMENT '空间id',
    `target_id`         bigint unsigned NOT NULL COMMENT '评测对象id',
    `target_version_id` bigint unsigned NOT NULL COMMENT '版本ID',
    `experiment_run_id` bigint unsigned NOT NULL COMMENT '实验执行id',
    `item_id`           bigint unsigned NOT NULL COMMENT '评测集行id',
    `turn_id`           bigint unsigned NOT NULL COMMENT '评测集行轮次id',
    `log_id`            varchar(255)    NOT NULL COMMENT 'log id',
    `trace_id`          varchar(255)    NOT NULL COMMENT 'trace id',
    `input_data`        mediumblob COMMENT '输入, json',
    `output_data`       mediumblob COMMENT '输出, json',
    `status`            int             NOT NULL COMMENT '执行状态',
    `created_at`        timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`        timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`        timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;评估对象记录信息';