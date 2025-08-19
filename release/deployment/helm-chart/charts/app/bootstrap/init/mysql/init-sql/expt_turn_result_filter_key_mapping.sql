CREATE TABLE IF NOT EXISTS `expt_turn_result_filter_key_mapping`
(
    `id`         bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键',
    `space_id`   bigint                                  NOT NULL COMMENT '空间id',
    `expt_id`    bigint                                  NOT NULL COMMENT '实验id',
    `from_field` varchar(255) COLLATE utf8mb4_general_ci NOT NULL COMMENT '筛选项唯一键，评估器: evaluator_version_id，人工标准：tag_key_id',
    `to_key`     varchar(255) COLLATE utf8mb4_general_ci NOT NULL COMMENT 'ck侧的map key，评估器：key1 ~ key10，人工标准：key1 ~ key100',
    `field_type` int                                     NOT NULL COMMENT '映射类型，Evaluator —— 1，人工标注—— 2',
    `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
    `created_by` varchar(255) COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '创建人',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_idx_space_expt_from_type` (`space_id`,`expt_id`,`field_type`,`from_field`)
) ENGINE=InnoDB AUTO_INCREMENT=6690 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='expt_turn_result_filter二级key映射表';