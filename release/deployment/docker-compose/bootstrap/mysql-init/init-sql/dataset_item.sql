CREATE TABLE IF NOT EXISTS `dataset_item`
(
    `id`              bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `app_id`          bigint unsigned NOT NULL DEFAULT '0' COMMENT '应用 ID',
    `space_id`        bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 ID',
    `dataset_id`      bigint unsigned NOT NULL DEFAULT '0' COMMENT '数据集 ID',
    `schema_id`       bigint unsigned NOT NULL DEFAULT '0' COMMENT 'Schema ID',
    `item_id`         bigint unsigned NOT NULL DEFAULT '0' COMMENT '条目 ID',
    `item_key`        varchar(128)    NOT NULL DEFAULT '' COMMENT '幂等 key',
    `data`            json                     DEFAULT NULL COMMENT '数据内容',
    `repeated_data`   json                     DEFAULT NULL COMMENT '多轮数据内容',
    `data_properties` json                     DEFAULT NULL COMMENT '内容属性',
    `add_vn`          bigint unsigned NOT NULL DEFAULT '0' COMMENT '添加版本号',
    `del_vn`          bigint unsigned NOT NULL DEFAULT '0' COMMENT '删除版本号',
    `created_by`      varchar(128)    NOT NULL DEFAULT '' COMMENT '创建人',
    `created_at`      timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_by`      varchar(128)    NOT NULL DEFAULT '' COMMENT '修改人',
    `updated_at`      timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    `deleted_at`      bigint          NOT NULL DEFAULT '0' COMMENT '删除时间',
    `update_version`  bigint unsigned NOT NULL DEFAULT '0' COMMENT '更新版本号，用于乐观锁',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_dataset_add_vn_item_id_deleted_at` (`dataset_id`, `add_vn`, `item_id`, `deleted_at`),
    UNIQUE KEY `uk_dataset_add_vn_item_key_deleted_at` (`dataset_id`, `add_vn`, `item_key`, `deleted_at`),
    KEY `idx_dataset_del_vn_created_at_item` (`dataset_id`, `del_vn`, `created_at`, `item_id`),
    KEY `idx_dataset_del_vn_updated_at_item` (`dataset_id`, `del_vn`, `updated_at`, `item_id`),
    KEY `idx_dataset_add_vn_del_vn_item` (`dataset_id`, `add_vn`, `del_vn`, `item_id`),
    KEY `idx_dataset_del_vn_item_id` (`dataset_id`, `del_vn`, `item_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;数据集条目';