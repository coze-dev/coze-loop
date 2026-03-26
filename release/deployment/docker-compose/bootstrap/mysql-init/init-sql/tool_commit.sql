CREATE TABLE IF NOT EXISTS `tool_commit` (
    `id`               bigint unsigned                         NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `space_id`         bigint unsigned                         NOT NULL COMMENT '空间ID',
    `tool_id`          bigint unsigned                         NOT NULL COMMENT 'Tool ID',
    `content`          longtext COLLATE utf8mb4_general_ci COMMENT '工具内容',
    `version`          varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '版本',
    `base_version`     varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '来源版本',
    `committed_by`     varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '提交人',
    `description`      text COLLATE utf8mb4_general_ci COMMENT '提交版本描述',
    `created_at`       datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       datetime                                NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_tool_version` (`tool_id`, `version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='工具版本';
