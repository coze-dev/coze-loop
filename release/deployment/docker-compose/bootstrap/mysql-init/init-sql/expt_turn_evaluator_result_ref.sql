CREATE TABLE IF NOT EXISTS `expt_turn_evaluator_result_ref`
(
    `id`                   bigint unsigned  NOT NULL DEFAULT '0' COMMENT 'id',
    `space_id`             bigint unsigned  NOT NULL COMMENT '空间 id',
    `expt_turn_result_id`  bigint unsigned  NOT NULL COMMENT '实验 turn result id',
    `evaluator_version_id` bigint unsigned  NOT NULL COMMENT '评估器版本 id; Inline 行写 0 哨兵',
    `source_type`          tinyint unsigned NOT NULL DEFAULT '0' COMMENT '0=旧数据(语义同 Builtin) / 1=Builtin / 2=Inline',
    `inline_key`           varchar(64)      NOT NULL DEFAULT '' COMMENT '仅 Inline: target output __inline_evaluators__ 的 key',
    `alias`                varchar(64)      NOT NULL DEFAULT '' COMMENT '仅 Builtin 别名实例; 与 inline_key 至多一个非空',
    `evaluator_result_id`  bigint unsigned  NOT NULL COMMENT '评估器结果 id',
    `created_at`           timestamp        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`           timestamp        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`           timestamp        NULL     DEFAULT NULL COMMENT '删除时间',
    `expt_id`              bigint unsigned  NOT NULL DEFAULT '0' COMMENT '实验 id',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_turn_result_evaluator_slot` (`space_id`, `expt_id`, `expt_turn_result_id`, `source_type`, `evaluator_version_id`, `inline_key`, `alias`),
    KEY `idx_turn_evaluator_result` (`space_id`, `expt_turn_result_id`, `evaluator_result_id`),
    KEY `idx_turn_evaluator_version` (`space_id`, `expt_turn_result_id`, `evaluator_version_id`),
    KEY `idx_expt_evaluator_result` (`space_id`, `expt_id`, `evaluator_result_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='expt_turn_evaluator_result_ref';
