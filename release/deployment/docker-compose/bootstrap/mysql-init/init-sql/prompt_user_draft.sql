CREATE TABLE IF NOT EXISTS `prompt_user_draft`
(
    `id`               bigint unsigned                         NOT NULL COMMENT '主键ID',
    `space_id`         bigint unsigned                         NOT NULL COMMENT '空间ID',
    `prompt_id`        bigint unsigned                         NOT NULL COMMENT 'Prompt ID',
    `user_id`          varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '用户ID',
    `template_type`    varchar(64) COLLATE utf8mb4_general_ci           DEFAULT 'Normal' COMMENT '模版类型',
    `messages`         longtext COLLATE utf8mb4_general_ci COMMENT '托管消息列表',
    `model_config`     text COLLATE utf8mb4_general_ci COMMENT '模型配置',
    `variable_defs`    text COLLATE utf8mb4_general_ci COMMENT '变量定义',
    `tools`            longtext COLLATE utf8mb4_general_ci COMMENT 'tools',
    `tool_call_config` text COLLATE utf8mb4_general_ci COMMENT 'tool调用配置',
    `base_version`     varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '草稿关联版本',
    `is_draft_edited`  tinyint                                 NOT NULL DEFAULT '0' COMMENT '草稿内容是否基于BaseVersion有变更',
    `created_at`       datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`       bigint                                  NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_prompt_id_user_id_deleted_at` (`prompt_id`, `user_id`, `deleted_at`),
    KEY `idx_prompt_id_user_id` (`prompt_id`, `user_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='Draft表';