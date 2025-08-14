CREATE TABLE IF NOT EXISTS `expt_evaluator_ref`
(
    `id`                   bigint unsigned NOT NULL DEFAULT '0' COMMENT 'id',
    `space_id`             bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 id',
    `expt_id`              bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验 id',
    `evaluator_id`         bigint unsigned NOT NULL DEFAULT '0' COMMENT '评估器 id',
    `evaluator_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '评估器版本 id',
    `created_at`           timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`           timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`           timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    PRIMARY KEY (`id`),
    KEY `idx_space_expt` (`space_id`, `expt_id`),
    KEY `idx_space_evaluator` (`space_id`, `evaluator_id`),
    KEY `idx_space_evaluator_version` (`space_id`, `evaluator_version_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='expt_evaluator_ref';