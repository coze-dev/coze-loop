CREATE TABLE IF NOT EXISTS `prompt_commit`
(
    `id`               bigint unsigned                         NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `space_id`         bigint unsigned                         NOT NULL COMMENT '空间ID',
    `prompt_id`        bigint unsigned                         NOT NULL COMMENT 'Prompt ID',
    `prompt_key`       varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT 'Prompt key',
    `template_type`    varchar(64) COLLATE utf8mb4_general_ci           DEFAULT 'normal' COMMENT '模版类型',
    `messages`         longtext COLLATE utf8mb4_general_ci COMMENT '托管消息列表',
    `model_config`     text COLLATE utf8mb4_general_ci COMMENT '模型配置',
    `variable_defs`    text COLLATE utf8mb4_general_ci COMMENT '变量定义',
    `tools`            longtext COLLATE utf8mb4_general_ci COMMENT 'tools',
    `tool_call_config` text COLLATE utf8mb4_general_ci COMMENT 'tool调用配置',
    `version`          varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '版本',
    `base_version`     varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '来源版本',
    `committed_by`     varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '提交人',
    `description`      text COLLATE utf8mb4_general_ci COMMENT '提交版本描述',
    `created_at`       datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_prompt_id_version` (`prompt_id`, `version`),
    KEY `idx_prompt_key_version` (`prompt_key`, `version`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='Commit表';