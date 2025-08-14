CREATE TABLE IF NOT EXISTS `expt_run_log`
(
    `id`             bigint unsigned NOT NULL DEFAULT '0' COMMENT 'id',
    `space_id`       bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 id',
    `created_by`     varchar(128)    NOT NULL DEFAULT '' COMMENT '创建者 id',
    `expt_id`        bigint          NOT NULL COMMENT '实验 id',
    `expt_run_id`    bigint          NOT NULL COMMENT '运行 id',
    `item_ids`       blob COMMENT '组 ids',
    `mode`           int                      DEFAULT NULL COMMENT '模式',
    `status`         bigint                   DEFAULT NULL COMMENT '状态',
    `pending_cnt`    int unsigned    NOT NULL DEFAULT '0' COMMENT 'item 未执行数量',
    `success_cnt`    int unsigned    NOT NULL DEFAULT '0' COMMENT 'item 成功数量',
    `fail_cnt`       int unsigned    NOT NULL DEFAULT '0' COMMENT 'item 失败数量',
    `credit_cost`    decimal(15, 2)  NOT NULL DEFAULT '0.00' COMMENT 'credit 消耗',
    `token_cost`     bigint                   DEFAULT NULL COMMENT 'token 消耗',
    `created_at`     timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`     timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`     timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    `status_message` blob COMMENT '提示信息',
    `processing_cnt` int             NOT NULL DEFAULT '0' COMMENT 'processing_cnt',
    `terminated_cnt` int             NOT NULL DEFAULT '0' COMMENT 'terminated_cnt',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_expt_run` (`space_id`, `expt_id`, `expt_run_id`),
    KEY `idx_expt_run_item_turn` (`space_id`, `expt_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='expt_run_log';