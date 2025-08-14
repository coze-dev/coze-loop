CREATE TABLE IF NOT EXISTS `dataset_schema`
(
    `id`             bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `app_id`         int unsigned    NOT NULL DEFAULT '0' COMMENT '应用 ID',
    `space_id`       bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 ID',
    `dataset_id`     bigint unsigned NOT NULL DEFAULT '0' COMMENT '数据集 ID',
    `fields`         json            NOT NULL COMMENT '字段格式',
    `immutable`      tinyint(1)      NOT NULL DEFAULT '0' COMMENT '是否不允许编辑',
    `created_by`     varchar(128)    NOT NULL DEFAULT '' COMMENT '创建人',
    `created_at`     timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_by`     varchar(128)    NOT NULL DEFAULT '' COMMENT '修改人',
    `updated_at`     timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    `update_version` bigint unsigned NOT NULL DEFAULT '0' COMMENT '更新版本号',
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;数据集 Schema';
