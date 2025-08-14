CREATE TABLE IF NOT EXISTS `expt_aggr_result`
(
    `id`            bigint unsigned NOT NULL COMMENT 'idgen id',
    `space_id`      bigint unsigned NOT NULL COMMENT '空间id',
    `experiment_id` bigint unsigned NOT NULL COMMENT '实验id',
    `field_type`    int                      DEFAULT NULL COMMENT '聚合字段类型 1：评估器得分',
    `field_key`     varchar(255)    NOT NULL COMMENT '聚合字段唯一标识',
    `score`         decimal(10, 4)           DEFAULT NULL COMMENT '聚合后的平均得分',
    `aggr_result`   blob COMMENT '详细聚合结果',
    `version`       bigint unsigned NOT NULL DEFAULT '0' COMMENT '版本号(用于乐观锁)',
    `status`        int             NOT NULL COMMENT '计算状态 1:idle 2: caculating',
    `created_at`    timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`    timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`    timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_experiment_id_field_type_field_key` (`experiment_id`, `field_type`, `field_key`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='实验聚合结果表';