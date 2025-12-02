CREATE TABLE IF NOT EXISTS `prompt_relation` (
    `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `space_id` bigint unsigned NOT NULL COMMENT '空间ID',
    `main_prompt_id` bigint unsigned NOT NULL COMMENT '主Prompt ID',
    `main_prompt_version` varchar(128) NOT NULL DEFAULT '' COMMENT '主Prompt版本',
    `main_draft_user_id` varchar(128) NOT NULL DEFAULT '' COMMENT '主Prompt草稿Owner',
    `sub_prompt_id` bigint unsigned NOT NULL COMMENT '子Prompt ID',
    `sub_prompt_version` varchar(128) NOT NULL DEFAULT '' COMMENT '子Prompt版本',
    `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_main_prompt_id_version` (`main_prompt_id`,`main_prompt_version`) COMMENT '主prompt_id_版本',
    KEY `idx_main_prompt_id_user` (`main_prompt_id`,`main_draft_user_id`) COMMENT '主prompt_id_user',
    KEY `idx_sub_prompt_id_version_create_time` (`sub_prompt_id`,`sub_prompt_version`, `create_time`) COMMENT '子prompt_id_版本'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Prompt关联表';