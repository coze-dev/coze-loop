CREATE TABLE IF NOT EXISTS `prompt_commit_label_mapping`
(
    `id`                 bigint unsigned                   NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `space_id`           bigint unsigned                   NOT NULL COMMENT '空间ID',
    `prompt_id`          bigint unsigned                   NOT NULL COMMENT 'Prompt ID',
    `prompt_key`          varchar(128) COLLATE utf8mb4_bin  NOT NULL COMMENT 'Prompt Key',
    `label_key`          varchar(128) COLLATE utf8mb4_bin  NOT NULL COMMENT 'Label唯一标识',
    `prompt_version`     varchar(128) COLLATE utf8mb4_bin  NOT NULL COMMENT 'Prompt版本',
    `created_by`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '创建人',
    `created_at`         datetime                          NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_by`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '更新人',
    `updated_at`         datetime                          NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`         bigint                            NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_prompt_id_label_key_deleted_at` (`prompt_id`, `label_key`, `deleted_at`),
    KEY `idx_prompt_id_version` (`prompt_id`, `prompt_version`) USING BTREE,
    KEY `idx_created_at` (`created_at`) USING BTREE
) ENGINE = InnoDB
    DEFAULT CHARSET = utf8mb4
    COLLATE = utf8mb4_general_ci COMMENT ='Prompt提交版本和Label关联表';
