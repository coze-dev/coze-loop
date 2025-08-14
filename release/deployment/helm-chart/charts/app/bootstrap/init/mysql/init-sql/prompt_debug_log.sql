CREATE TABLE IF NOT EXISTS `prompt_debug_log`
(
    `id`            bigint unsigned                         NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `prompt_id`     bigint unsigned                         NOT NULL DEFAULT '0' COMMENT 'Prompt ID',
    `space_id`      bigint unsigned                         NOT NULL DEFAULT '0' COMMENT '空间ID',
    `prompt_key`    varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT 'prompt key',
    `version`       varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT 'version',
    `input_tokens`  bigint                                  NOT NULL DEFAULT '0' COMMENT 'input_tokens',
    `output_tokens` bigint                                  NOT NULL DEFAULT '0' COMMENT 'output_tokens',
    `started_at`    bigint unsigned                                  DEFAULT '0' COMMENT '请求开始毫秒时间戳',
    `ended_at`      bigint unsigned                                  DEFAULT '0' COMMENT '响应结束毫秒时间戳',
    `cost_ms`       bigint unsigned                                  DEFAULT '0' COMMENT '响应耗时毫秒',
    `status_code`   int                                              DEFAULT NULL COMMENT '状态码',
    `debugged_by`   varchar(128) COLLATE utf8mb4_general_ci          DEFAULT '0' COMMENT '执行人UserID',
    `debug_id`      bigint unsigned                         NOT NULL DEFAULT '0' COMMENT 'debug_id',
    `debug_step`    int                                     NOT NULL DEFAULT '1' COMMENT 'debug_step',
    `created_at`    datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`    datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`    bigint                                  NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`),
    KEY `idx_prompt_id_debugged_by_started_at` (`prompt_id`, `debugged_by`, `started_at`) USING BTREE,
    KEY `idx_debug_id_step` (`debug_id`, `debug_step`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='debug表';