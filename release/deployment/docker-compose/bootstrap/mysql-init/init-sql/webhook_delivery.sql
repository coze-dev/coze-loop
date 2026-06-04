CREATE TABLE IF NOT EXISTS `webhook_delivery`
(
    `id`             bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
    `space_id`       bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 ID',
    `expt_id`        bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验 ID',
    `delivery_id`    varchar(64)     NOT NULL DEFAULT '' COMMENT '投递唯一 ID',
    `event_type`     varchar(64)     NOT NULL DEFAULT '' COMMENT '触发事件类型',
    `channel_type`   varchar(16)     NOT NULL DEFAULT 'webhook' COMMENT '渠道类型',
    `webhook_url`    varchar(2048)   NOT NULL DEFAULT '' COMMENT 'Webhook 目标 URL',
    `status`         varchar(16)     NOT NULL DEFAULT 'pending' COMMENT '投递状态',
    `attempt_count`  int unsigned    NOT NULL DEFAULT '1' COMMENT '已发送次数',
    `max_attempts`   int unsigned    NOT NULL DEFAULT '4' COMMENT '最大发送次数',
    `first_sent_at`  timestamp       NULL DEFAULT NULL COMMENT '首次发送时间',
    `last_sent_at`   timestamp       NULL DEFAULT NULL COMMENT '最近一次发送时间',
    `next_retry_at`  timestamp       NULL DEFAULT NULL COMMENT '下次重试时间',
    `response_code`  int             NULL DEFAULT NULL COMMENT '最近一次 HTTP 响应状态码',
    `error_message`  varchar(1024)   NOT NULL DEFAULT '' COMMENT '最近一次失败原因',
    `created_by`     varchar(128)    NOT NULL DEFAULT '' COMMENT '创建者 ID',
    `updated_by`     varchar(128)    NOT NULL DEFAULT '' COMMENT '更新者 ID',
    `deleted_at`     bigint          NOT NULL DEFAULT '0' COMMENT '删除时间',
    `created_at`     datetime        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`     datetime        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_delivery_id` (`delivery_id`),
    KEY `idx_space_expt_id` (`space_id`, `expt_id`),
    KEY `idx_status_next_retry` (`status`, `next_retry_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='webhook_delivery';
