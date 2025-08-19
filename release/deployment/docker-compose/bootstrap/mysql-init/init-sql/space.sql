CREATE TABLE IF NOT EXISTS `space`
(
    `id`          bigint(20) unsigned NOT NULL COMMENT 'Primary Key ID, Space ID',
    `owner_id`    bigint(20) unsigned NOT NULL DEFAULT 0 COMMENT 'Owner ID',
    `name`        varchar(200)        NOT NULL DEFAULT '' COMMENT 'Space Name',
    `description` varchar(2000)       NOT NULL DEFAULT '' COMMENT 'Space Description',
    `space_type`  tinyint(4)          NOT NULL DEFAULT '0' COMMENT 'Space Type, 1: Personal, 2: Team',
    `icon_uri`    varchar(200)        NOT NULL DEFAULT '' COMMENT 'Icon URI',
    `created_by`  bigint(20) unsigned NOT NULL DEFAULT 0 COMMENT 'Creator ID',
    `deleted_at`  bigint              NOT NULL DEFAULT '0' COMMENT '删除时间',
    `created_at`  datetime            NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`  datetime            NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_owner_id` (`owner_id`),
    KEY `idx_creator_id` (`created_by`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT = 'Space Table';
