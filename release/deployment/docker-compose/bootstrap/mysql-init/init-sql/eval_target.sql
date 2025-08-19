CREATE TABLE IF NOT EXISTS `eval_target`
(
    `id`               bigint unsigned NOT NULL COMMENT 'idgen id',
    `space_id`         bigint unsigned NOT NULL COMMENT '空间id',
    `source_target_id` varchar(255)    NOT NULL COMMENT '来源的对象的ID，比如promptID',
    `target_type`      int unsigned    NOT NULL COMMENT '评估对象类型',
    `created_by`       varchar(128)    NOT NULL DEFAULT '0' COMMENT '创建人',
    `updated_by`       varchar(128)    NOT NULL DEFAULT '0' COMMENT '更新人',
    `created_at`       timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`       timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_space_id_source_target_id_target_type` (`space_id`, `source_target_id`, `target_type`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;评估对象信息';