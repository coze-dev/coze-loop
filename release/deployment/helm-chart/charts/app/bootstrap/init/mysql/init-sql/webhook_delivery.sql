CREATE TABLE IF NOT EXISTS `webhook_delivery`
(
    `id`               bigint unsigned                                               NOT NULL COMMENT 'id',
    `space_id`         bigint unsigned                                               NOT NULL DEFAULT '0' COMMENT '空间 id',
    `delivery_id`      varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '投递唯一ID(UUID)',
    `experiment_id`    bigint unsigned                                               NOT NULL DEFAULT '0' COMMENT '实验 id',
    `event_type`       varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '触发事件类型: started/succeeded/failed/terminated',
    `webhook_url`      varchar(2048) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT 'Webhook URL',
    `status`           int unsigned                                                  NOT NULL DEFAULT '0' COMMENT '投递状态: 0-pending, 1-success, 2-failed, 3-retrying',
    `retry_count`      int unsigned                                                  NOT NULL DEFAULT '0' COMMENT '已重试次数',
    `last_status_code` int                                                           NOT NULL DEFAULT '0' COMMENT '最近一次HTTP响应状态码',
    `error_message`    varchar(1024) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '失败原因',
    `request_headers`  blob COMMENT '请求头JSON',
    `next_retry_at`    timestamp                                                     NULL     DEFAULT NULL COMMENT '下次重试时间',
    `first_sent_at`    timestamp                                                     NULL     DEFAULT NULL COMMENT '首次发送时间',
    `last_sent_at`     timestamp                                                     NULL     DEFAULT NULL COMMENT '最近发送时间',
    `created_at`       timestamp                                                     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       timestamp                                                     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_delivery_id` (`delivery_id`),
    KEY `idx_space_expt_id` (`space_id`, `experiment_id`),
    KEY `idx_status_next_retry` (`status`, `next_retry_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='webhook_delivery';
