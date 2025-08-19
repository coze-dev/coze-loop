CREATE TABLE IF NOT EXISTS `expt_stats`
(
    `id`                bigint unsigned NOT NULL DEFAULT '0' COMMENT 'id',
    `space_id`          bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 id',
    `expt_id`           bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验 id',
    `pending_cnt`       int             NOT NULL DEFAULT '0' COMMENT 'pending_cnt',
    `success_cnt`       int             NOT NULL DEFAULT '0' COMMENT 'success_cnt',
    `fail_cnt`          int             NOT NULL DEFAULT '0' COMMENT 'fail_cnt',
    `credit_cost`       decimal(15, 2)  NOT NULL DEFAULT '0.00' COMMENT 'credit 消耗',
    `input_token_cost`  bigint                   DEFAULT NULL COMMENT 'input token 消耗',
    `output_token_cost` bigint                   DEFAULT NULL COMMENT 'output token 消耗',
    `created_at`        timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`        timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`        timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    `processing_cnt`    int             NOT NULL DEFAULT '0' COMMENT 'processing_cnt',
    `terminated_cnt`    int             NOT NULL DEFAULT '0' COMMENT 'terminated_cnt',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_space_expt` (`space_id`, `expt_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='expt_stats';