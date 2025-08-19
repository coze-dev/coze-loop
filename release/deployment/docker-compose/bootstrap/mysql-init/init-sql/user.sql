CREATE TABLE IF NOT EXISTS `user`
(
    `id`            bigint(20)   NOT NULL COMMENT 'Primary Key ID',
    `name`          varchar(128) NOT NULL DEFAULT '' COMMENT 'User Nickname',
    `unique_name`   varchar(128) NOT NULL DEFAULT '' COMMENT 'User Unique Name',
    `email`         varchar(128) NOT NULL DEFAULT '' COMMENT 'Email',
    `password`      varchar(128) NOT NULL DEFAULT '' COMMENT 'Password (Encrypted)',
    `description`   varchar(512) NOT NULL DEFAULT '' COMMENT 'User Description',
    `icon_uri`      varchar(512) NOT NULL DEFAULT '' COMMENT 'Avatar URI',
    `user_verified` tinyint(1)   NOT NULL DEFAULT 0 COMMENT 'User Verification Status',
    `country_code`  bigint(20)   NOT NULL DEFAULT 0 COMMENT 'Country Code',
    `session_key`   varchar(512) NOT NULL DEFAULT '' COMMENT 'Session Key',
    `deleted_at`    bigint       NOT NULL DEFAULT '0' COMMENT '删除时间',
    `created_at`    datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`    datetime     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_unique_name` (`unique_name`),
    UNIQUE KEY `idx_email` (`email`),
    KEY `idx_session_key` (`session_key`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT = 'User Table';
