CREATE TABLE IF NOT EXISTS `expt_evaluator_ref`
(
    `id`                   bigint unsigned NOT NULL DEFAULT '0' COMMENT 'id',
    `space_id`             bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 id',
    `expt_id`              bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验 id',
    `eval_set_id`          bigint unsigned NOT NULL DEFAULT '0' COMMENT '该 binding 归属的评测集 id(反查标签); 0=老数据/单 set',
    `evaluator_id`         bigint unsigned NOT NULL DEFAULT '0' COMMENT '评估器 id',
    `evaluator_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '评估器版本 id',
    `alias`                varchar(64)     NOT NULL DEFAULT '' COMMENT '别名: 同 (evaluator_id, evaluator_version_id) 多实例区分(judge_A/judge_B); 默认实例为空串',
    `filter`               blob            COMMENT '行级过滤配置快照, json: {filter_fields: [...], filter_mode: 0 None/1 Include/2 Exclude}; 仅供查询',
    `binding_config`       blob            COMMENT 'binding 配置快照, json: {IngressConf, RunConf, ScoreWeight}; 仅供查询',
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
