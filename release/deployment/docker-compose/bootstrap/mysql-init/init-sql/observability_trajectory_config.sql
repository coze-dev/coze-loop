CREATE TABLE IF NOT EXISTS  `observability_trajectory_config`
(
    `id`             bigint unsigned                          NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `workspace_id`   bigint unsigned                          NOT NULL DEFAULT '0' COMMENT '空间 ID',
    `filter`         json                                     DEFAULT NULL COMMENT 'trace展示的过滤配置',
    `created_at`     datetime                                 NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`     varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_at`     datetime                                 NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    `updated_by`     varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '修改人',
    `is_deleted`     tinyint(1)                               NOT NULL DEFAULT '0' COMMENT '是否删除, 0 表示未删除, 1 表示已删除',
    `deleted_at`     datetime                                          DEFAULT NULL COMMENT '删除时间',
    `deleted_by`     varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '删除人',
    PRIMARY KEY (`id`),
    KEY `idx_space_id` (`workspace_id`)
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_general_ci COMMENT='观测轨迹配置';