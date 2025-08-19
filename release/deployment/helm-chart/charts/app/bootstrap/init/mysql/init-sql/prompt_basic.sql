CREATE TABLE IF NOT EXISTS `prompt_basic`
(
    `id`                 bigint unsigned                   NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `space_id`           bigint unsigned                   NOT NULL COMMENT '空间ID',
    `prompt_key`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT 'Prompt key',
    `name`               varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT 'Prompt名称',
    `description`        varchar(1024) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `created_by`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_by`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '更新人',
    `commit_status`      tinyint                           NOT NULL DEFAULT '0' COMMENT '提交状态',
    `latest_version`     varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '最新版本',
    `latest_commit_time` datetime                                   DEFAULT NULL COMMENT '最新提交时间',
    `created_at`         datetime                          NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`         datetime                          NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`         bigint                            NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_space_id_prompt_key_deleted_at` (`space_id`, `prompt_key`, `deleted_at`),
    KEY `idx_created_at` (`created_at`) USING BTREE
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='Prompt基础表';