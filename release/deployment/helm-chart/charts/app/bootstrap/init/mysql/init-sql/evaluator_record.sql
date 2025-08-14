CREATE TABLE IF NOT EXISTS `evaluator_record`
(
    `id`                   bigint unsigned NOT NULL COMMENT 'idgen id',
    `space_id`             bigint unsigned NOT NULL COMMENT '空间id',
    `evaluator_version_id` bigint unsigned NOT NULL COMMENT '评估器版本id',
    `experiment_id`        bigint unsigned          DEFAULT NULL COMMENT '实验id',
    `experiment_run_id`    bigint unsigned NOT NULL COMMENT '实验执行id',
    `item_id`              bigint unsigned NOT NULL COMMENT '评估集行id',
    `turn_id`              bigint unsigned NOT NULL DEFAULT '0' COMMENT '评估集行轮次id',
    `log_id`               varchar(255)             DEFAULT NULL COMMENT 'log id',
    `trace_id`             varchar(255)    NOT NULL COMMENT 'trace id',
    `score`                decimal(10, 4)           DEFAULT NULL COMMENT '得分',
    `status`               int             NOT NULL COMMENT '执行状态',
    `created_at`           timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`           timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`           timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    `input_data`           mediumblob COMMENT '输入, json',
    `output_data`          mediumblob COMMENT '执行结果, json',
    `created_by`           varchar(128)    NOT NULL DEFAULT '0' COMMENT '创建人',
    `updated_by`           varchar(128)    NOT NULL DEFAULT '0' COMMENT '更新人',
    `ext`                  mediumblob COMMENT '补充信息, json',
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;评估器执行结果';