CREATE TABLE IF NOT EXISTS `expt_insight_analysis_feedback_vote` (
                                                       `id` bigint unsigned NOT NULL COMMENT '唯一标识 idgen生成',
                                                       `space_id` bigint unsigned NOT NULL COMMENT 'SpaceID',
                                                       `expt_id` bigint unsigned NOT NULL COMMENT 'exptID',
                                                       `vote_type` int NOT NULL COMMENT '反馈类型',
                                                       `analysis_record_id` bigint unsigned COMMENT '洞察分析记录ID',
                                                       `created_by` varchar(128) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '创建者 id',
                                                       `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                                       `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                                       `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
                                                       PRIMARY KEY (`id`),
                                                       UNIQUE KEY `uniq_space_id_expt_id_analysis_record_id_created_by` (`space_id`,`expt_id`,`analysis_record_id`,`created_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='实验洞察分析反馈点赞表';