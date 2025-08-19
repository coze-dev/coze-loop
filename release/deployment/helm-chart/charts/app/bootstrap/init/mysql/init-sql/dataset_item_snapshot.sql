CREATE TABLE IF NOT EXISTS `dataset_item_snapshot`
(
    `id`              bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `app_id`          bigint unsigned NOT NULL DEFAULT '0' COMMENT '应用 ID',
    `space_id`        bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 ID',
    `dataset_id`      bigint unsigned NOT NULL DEFAULT '0' COMMENT '数据集 ID',
    `schema_id`       bigint unsigned NOT NULL DEFAULT '0' COMMENT 'Schema ID',
    `version_id`      bigint unsigned NOT NULL DEFAULT '0' COMMENT 'Version ID',
    `item_primary_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '条目主键 ID',
    `item_id`         bigint unsigned NOT NULL DEFAULT '0' COMMENT '条目 ID',
    `item_key`        varchar(128)    NOT NULL DEFAULT '' COMMENT '条目幂等 key',
    `data`            json                     DEFAULT NULL COMMENT '数据内容',
    `repeated_data`   json                     DEFAULT NULL COMMENT '多轮数据内容',
    `data_properties` json                     DEFAULT NULL COMMENT '内容属性',
    `add_vn`          bigint unsigned NOT NULL DEFAULT '0' COMMENT '添加版本号',
    `del_vn`          bigint unsigned NOT NULL DEFAULT '0' COMMENT '删除版本号',
    `created_at`      timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'snapshot 创建时间',
    `item_created_by` varchar(128)    NOT NULL DEFAULT '' COMMENT 'item 创建人',
    `item_created_at` timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'item 创建时间',
    `item_updated_by` varchar(128)    NOT NULL DEFAULT '' COMMENT 'item 修改人',
    `item_updated_at` timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'item 修改时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_version_item` (`version_id`, `item_id`),
    KEY `idx_version_item_created_at_item` (`version_id`, `item_created_at`, `item_id`),
    KEY `idx_version_item_updated_at_item` (`version_id`, `item_updated_at`, `item_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;数据集条目快照';