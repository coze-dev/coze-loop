CREATE TABLE IF NOT EXISTS `eval_target_version`
(
    `id`                    bigint unsigned NOT NULL COMMENT 'target version id',
    `space_id`              bigint unsigned NOT NULL COMMENT '空间id',
    `target_id`             bigint unsigned NOT NULL COMMENT 'target id',
    `source_target_version` varchar(255)    NOT NULL COMMENT 'source target version',
    `target_meta`           blob COMMENT '具体内容, 每种静态规则类型对应一个解析方式, json',
    `input_schema`          blob COMMENT '评估器输入结构信息, json',
    `output_schema`         blob COMMENT '评估器输出结构信息, json',
    `created_by`            varchar(128)    NOT NULL DEFAULT '0' COMMENT '创建人',
    `updated_by`            varchar(128)    NOT NULL DEFAULT '0' COMMENT '更新人',
    `created_at`            timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`            timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`            timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_space_id_target_id_source_target_version` (`space_id`, `target_id`, `source_target_version`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;评估对象版本信息';