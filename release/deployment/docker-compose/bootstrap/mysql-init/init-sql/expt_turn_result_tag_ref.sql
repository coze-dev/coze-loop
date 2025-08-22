CREATE TABLE IF NOT EXISTS `expt_turn_result_tag_ref` (
                                            `id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'id',
                                            `space_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 id',
                                            `expt_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验 id',
                                            `tag_key_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '标签 id',
                                            `total_cnt` int NOT NULL DEFAULT '0' COMMENT 'total_cnt',
                                            `complete_cnt` int NOT NULL DEFAULT '0' COMMENT 'complete_cnt',
                                            `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                            `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                            `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
                                            PRIMARY KEY (`id`),
                                            UNIQUE KEY `uniq_space_tag_key_id` (`space_id`,`expt_id`,`tag_key_id`),
                                            KEY `idx_space_expt` (`space_id`,`expt_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='expt_turn_result_tag_ref';