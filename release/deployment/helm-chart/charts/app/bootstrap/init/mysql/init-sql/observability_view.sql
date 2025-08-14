CREATE TABLE IF NOT EXISTS `observability_view`
(
    `id`             bigint unsigned                          NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `enterprise_id`  varchar(200) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '企业id',
    `workspace_id`   bigint unsigned                          NOT NULL DEFAULT '0' COMMENT '空间 ID',
    `view_name`      varchar(256) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '视图名称',
    `platform_type`  varchar(128)                             NOT NULL DEFAULT '' COMMENT '数据来源',
    `span_list_type` varchar(128)                             NOT NULL DEFAULT '' COMMENT '列表信息',
    `filters`        varchar(2048) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '过滤条件信息',
    `created_at`     datetime                                 NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`     varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_at`     datetime                                 NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    `updated_by`     varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '修改人',
    `is_deleted`     tinyint(1)                               NOT NULL DEFAULT '0' COMMENT '是否删除, 0 表示未删除, 1 表示已删除',
    `deleted_at`     datetime                                          DEFAULT NULL COMMENT '删除时间',
    `deleted_by`     varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '删除人',
    PRIMARY KEY (`id`),
    KEY `idx_space_id_created_by` (`workspace_id`, `created_by`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='观测视图信息';