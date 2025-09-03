CREATE TABLE IF NOT EXISTS `expt_result_export_record` (
                                             `id` bigint unsigned NOT NULL COMMENT 'export_id 导出的唯一标识 idgen生成',
                                             `space_id` bigint unsigned NOT NULL COMMENT 'SpaceID',
                                             `expt_id` bigint unsigned NOT NULL COMMENT 'exptID',
                                             `csv_export_status` int NOT NULL COMMENT 'CSV导出状态：1-导出中, 2-导出成功 3-导出失败',
                                             `file_path` varchar(255) COLLATE utf8mb4_general_ci NOT NULL COMMENT 'tos文件路径',
                                             `start_at` timestamp NULL DEFAULT NULL COMMENT '开始执行时间',
                                             `end_at` timestamp NULL DEFAULT NULL COMMENT '结束执行时间',
                                             `created_by` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '创建者 id',
                                             `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                             `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                             `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
                                             `err_msg` blob COMMENT '错误信息',
                                             PRIMARY KEY (`id`),
                                             KEY `idx_space_id_expt_id` (`space_id`,`expt_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='实验导出信息表';