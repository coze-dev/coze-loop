CREATE TABLE IF NOT EXISTS `expt_insight_analysis_record` (
                                                `id` bigint unsigned NOT NULL COMMENT '唯一标识 idgen生成',
                                                `space_id` bigint unsigned NOT NULL COMMENT 'SpaceID',
                                                `expt_id` bigint unsigned NOT NULL COMMENT 'exptID',
                                                `status` int NOT NULL COMMENT '状态',
                                                `expt_result_file_path` varchar(255) COMMENT '原始报告文件路径',
                                                `analysis_report_id` bigint unsigned COMMENT '洞察分析报告ID',
                                                `created_by` varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '创建者 id',
                                                `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                                `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                                `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
                                                PRIMARY KEY (`id`),
                                                KEY `idx_space_id_expt_id` (`space_id`,`expt_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='实验洞察分析记录表';