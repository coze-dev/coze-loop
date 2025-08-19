CREATE TABLE IF NOT EXISTS `api_key`
(
    `id`           bigint(20) unsigned NOT NULL COMMENT 'Primary Key ID',
    `key`          varchar(255)        NOT NULL DEFAULT '' COMMENT 'API Key hash',
    `name`         varchar(255)        NOT NULL DEFAULT '' COMMENT 'API Key Name',
    `status`       tinyint             NOT NULL DEFAULT 0 COMMENT '0 normal, 1 deleted',
    `user_id`      bigint(20)          NOT NULL DEFAULT '0' COMMENT 'API Key Owner',
    `expired_at`   bigint(20)          NOT NULL DEFAULT '0' COMMENT 'API Key Expired Time',
    `created_at`   datetime            NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Created Time',
    `updated_at`   datetime            NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Updated Time',
    `deleted_at`   bigint              NOT NULL DEFAULT '0' COMMENT 'Deleted Time',
    `last_used_at` bigint              NOT NULL DEFAULT '0' COMMENT 'Last Used Time',
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT = 'api key table';
