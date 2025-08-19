CREATE TABLE IF NOT EXISTS `evaluator_version`
(
    `id`                   bigint unsigned NOT NULL COMMENT 'idgen id',
    `space_id`             bigint unsigned NOT NULL COMMENT '空间id',
    `evaluator_type`       int unsigned             DEFAULT NULL COMMENT '评估器类型',
    `evaluator_id`         bigint unsigned NOT NULL COMMENT '评估器id',
    `version`              varchar(128)    NOT NULL COMMENT '版本号',
    `description`          varchar(500)             DEFAULT NULL COMMENT '版本描述',
    `metainfo`             blob COMMENT '具体内容, 每种静态规则类型对应一个解析方式, json',
    `receive_chat_history` tinyint(1)               DEFAULT '0' COMMENT '是否需求传递上下文',
    `input_schema`         blob COMMENT '评估器结构信息, json',
    `created_by`           varchar(128)    NOT NULL DEFAULT '0' COMMENT '创建人',
    `updated_by`           varchar(128)    NOT NULL DEFAULT '0' COMMENT '更新人',
    `created_at`           timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`           timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`           timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    PRIMARY KEY (`id`),
    KEY `idx_evaluator_id_version` (`evaluator_id`, `version`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;评估器版本信息';