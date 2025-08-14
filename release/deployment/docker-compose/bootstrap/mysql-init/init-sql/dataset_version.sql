CREATE TABLE IF NOT EXISTS `dataset_version`
(
    `id`                bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `app_id`            int unsigned    NOT NULL DEFAULT '0' COMMENT '应用 ID',
    `space_id`          bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 ID',
    `dataset_id`        bigint unsigned NOT NULL DEFAULT '0' COMMENT '数据集 ID',
    `schema_id`         bigint unsigned NOT NULL DEFAULT '0' COMMENT 'Schema ID',
    `dataset_brief`     json                     DEFAULT NULL COMMENT '数据集元信息备份',
    `version`           varchar(64)     NOT NULL DEFAULT '' COMMENT '版本号，SemVer2 三段式',
    `version_num`       bigint unsigned NOT NULL DEFAULT '1' COMMENT '数字版本号，从1开始递增',
    `description`       varchar(2048)   NOT NULL DEFAULT '' COMMENT '版本描述',
    `item_count`        bigint unsigned NOT NULL DEFAULT '0' COMMENT '条数',
    `snapshot_status`   varchar(64)     NOT NULL DEFAULT '' COMMENT '快照状态',
    `snapshot_progress` json                     DEFAULT NULL COMMENT '快照进度详情',
    `update_version`    bigint unsigned NOT NULL DEFAULT '0' COMMENT '更新版本号',
    `created_by`        varchar(128)    NOT NULL DEFAULT '' COMMENT '创建人',
    `created_at`        timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `disabled_at`       timestamp       NULL     DEFAULT NULL COMMENT '版本禁用时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_dataset_id_version` (`dataset_id`, `version`),
    KEY `idx_dataset_id_created_at_id` (`dataset_id`, `created_at`, `id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;数据集版本';