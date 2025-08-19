CREATE TABLE IF NOT EXISTS `evaluator`
(
    `id`              bigint unsigned NOT NULL COMMENT 'idgen id',
    `space_id`        bigint unsigned NOT NULL COMMENT '空间id',
    `evaluator_type`  int unsigned    NOT NULL COMMENT '评估器类型',
    `name`            varchar(255)             DEFAULT NULL COMMENT '名称',
    `description`     varchar(500)             DEFAULT NULL COMMENT '描述',
    `draft_submitted` tinyint(1)               DEFAULT '0' COMMENT '草稿是否已提交',
    `created_by`      varchar(128)    NOT NULL DEFAULT '0' COMMENT '创建人',
    `updated_by`      varchar(128)    NOT NULL DEFAULT '0' COMMENT '更新人',
    `created_at`      timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`      timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`      timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    `latest_version`  varchar(128)    NOT NULL DEFAULT '' COMMENT '最新版本号',
    PRIMARY KEY (`id`),
    KEY `idx_space_id_evaluator_type` (`space_id`, `evaluator_type`),
    KEY `idx_space_id_created_by` (`space_id`, `created_by`),
    KEY `idx_space_id_created_at` (`space_id`, `created_at`),
    KEY `idx_space_id_updated_at` (`space_id`, `updated_at`),
    KEY `idx_space_id_name` (`space_id`, `name`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;评估器信息';