CREATE TABLE IF NOT EXISTS `prompt_label`
(
    `id`                 bigint unsigned                   NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `space_id`           bigint unsigned                   NOT NULL COMMENT '空间ID',
    `label_key`          varchar(128) COLLATE utf8mb4_bin  NOT NULL COMMENT 'Label唯一标识',
    `created_by`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '创建人',
    `created_at`         datetime                          NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_by`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '更新人',
    `updated_at`         datetime                          NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`         bigint                            NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_space_id_label_key_deleted_at` (`space_id`, `label_key`, `deleted_at`),
    KEY `idx_created_at` (`created_at`) USING BTREE
) ENGINE = InnoDB
    DEFAULT CHARSET = utf8mb4
    COLLATE = utf8mb4_general_ci COMMENT ='Prompt Label表';
