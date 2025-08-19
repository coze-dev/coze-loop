CREATE TABLE IF NOT EXISTS `dataset`
(
    `id`               bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `app_id`           int unsigned    NOT NULL DEFAULT '0' COMMENT '应用 ID',
    `space_id`         bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 ID',
    `schema_id`        bigint unsigned NOT NULL DEFAULT '0' COMMENT 'Schema ID',
    `name`             varchar(255)    NOT NULL DEFAULT '' COMMENT '数据集名称',
    `description`      varchar(2048)   NOT NULL DEFAULT '' COMMENT '数据集描述',
    `category`         varchar(64)     NOT NULL DEFAULT '' COMMENT '业务场景分类',
    `biz_category`     varchar(128)    NOT NULL DEFAULT '' COMMENT '业务场景下自定义分类',
    `status`           varchar(128)    NOT NULL DEFAULT '' COMMENT '状态',
    `security_level`   varchar(32)     NOT NULL DEFAULT '' COMMENT '安全等级',
    `visibility`       varchar(64)     NOT NULL DEFAULT '' COMMENT '可见性',
    `spec`             json                     DEFAULT NULL COMMENT '规格配置',
    `features`         json                     DEFAULT NULL COMMENT '功能开关',
    `latest_version`   varchar(64)     NOT NULL DEFAULT '' COMMENT '最新版本号',
    `next_version_num` bigint unsigned NOT NULL DEFAULT '1' COMMENT '下一个版本的数字版本号',
    `last_operation`   varchar(255)    NOT NULL DEFAULT '' COMMENT '最新操作',
    `created_by`       varchar(128)    NOT NULL DEFAULT '' COMMENT '创建人',
    `created_at`       timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_by`       varchar(128)    NOT NULL DEFAULT '' COMMENT '修改人',
    `updated_at`       timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    `deleted_at`       bigint          NOT NULL DEFAULT '0' COMMENT '删除时间',
    `expired_at`       timestamp       NULL     DEFAULT NULL COMMENT '过期时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_space_id_category_name` (`space_id`, `category`, `name`, `deleted_at`),
    KEY `idx_space_id_category_updated_at_id` (`space_id`, `category`, `updated_at`, `id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;数据集';