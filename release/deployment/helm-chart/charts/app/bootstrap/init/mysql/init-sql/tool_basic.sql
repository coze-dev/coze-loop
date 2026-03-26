CREATE TABLE IF NOT EXISTS `tool_basic` (
    `id`                 bigint unsigned                   NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `space_id`           bigint unsigned                   NOT NULL COMMENT '空间ID',
    `name`               varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '名称',
    `description`        varchar(1024) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '描述',
    `latest_committed_version` varchar(128) COLLATE utf8mb4_bin NULL DEFAULT '' COMMENT '最新版本',
    `created_by`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_by`         varchar(128) COLLATE utf8mb4_bin  NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`         datetime                          NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`         datetime                          NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`         bigint                            NOT NULL DEFAULT '0' COMMENT '删除时间',
    PRIMARY KEY (`id`),
    KEY `idx_spaceid_name_delat` (`space_id`, `name`, `deleted_at`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='工具主体';
