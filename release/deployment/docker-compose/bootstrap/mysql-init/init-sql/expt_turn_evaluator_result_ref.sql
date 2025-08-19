CREATE TABLE IF NOT EXISTS `expt_turn_evaluator_result_ref`
(
    `id`                   bigint unsigned NOT NULL DEFAULT '0' COMMENT 'id',
    `space_id`             bigint unsigned NOT NULL COMMENT '空间 id',
    `expt_turn_result_id`  bigint unsigned NOT NULL COMMENT '实验 turn result id',
    `evaluator_version_id` bigint unsigned NOT NULL COMMENT '评估器版本 id',
    `evaluator_result_id`  bigint unsigned NOT NULL COMMENT '评估器结果 id',
    `created_at`           timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`           timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`           timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    `expt_id`              bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验 id',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_space_expt_turn_result_evaluator` (`space_id`, `expt_id`, `expt_turn_result_id`, `evaluator_version_id`),
    KEY `idx_turn_evaluator_result` (`space_id`, `expt_turn_result_id`, `evaluator_result_id`),
    KEY `idx_turn_evaluator_version` (`space_id`, `expt_turn_result_id`, `evaluator_version_id`),
    KEY `idx_expt_evaluator_result` (`space_id`, `expt_id`, `evaluator_result_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='expt_turn_evaluator_result_ref';