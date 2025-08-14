CREATE TABLE IF NOT EXISTS `prompt_debug_context`
(
    `id`             bigint unsigned                         NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `prompt_id`      bigint                                  NOT NULL DEFAULT '0' COMMENT 'prompt id',
    `user_id`        varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT 'user id',
    `mock_contexts`  longtext COLLATE utf8mb4_general_ci COMMENT '上下文信息，json格式',
    `mock_variables` longtext COLLATE utf8mb4_general_ci COMMENT 'mock变量值，json格式',
    `mock_tools`     longtext COLLATE utf8mb4_general_ci COMMENT 'mock tool结果，json格式',
    `debug_config`   text COLLATE utf8mb4_general_ci COMMENT '调试配置',
    `compare_config` longtext COLLATE utf8mb4_general_ci COMMENT '训练场配置',
    `created_at`     datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`     datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`     bigint                                  NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_prompt_id_user_id` (`prompt_id`, `user_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='用户调试prompt上下文信息表';