CREATE TABLE IF NOT EXISTS `annotate_record` (
                                   `id` bigint unsigned NOT NULL COMMENT 'idgen record id',
                                   `space_id` bigint unsigned NOT NULL COMMENT '空间id，分片键',
                                   `tag_key_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '标签 id',
                                   `experiment_id` bigint unsigned NOT NULL COMMENT '实验id',
                                   `score` decimal(10,4) DEFAULT NULL COMMENT '得分结果',
                                   `text_value` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '文本结果',
                                   `annotate_data` mediumblob COMMENT '标注结果, json',
                                   `created_at` bigint NOT NULL DEFAULT '0' COMMENT '创建时间',
                                   `updated_at` bigint NOT NULL DEFAULT '0' COMMENT '更新时间',
                                   `deleted_at` bigint NOT NULL DEFAULT '0' COMMENT '软删除时间',
                                   `created_by` bigint NOT NULL DEFAULT '0' COMMENT '创建人userID',
                                   `tag_value_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '标签值 id',
                                   PRIMARY KEY (`id`),
                                   KEY `idx_space_id_experiment_id_tag_key_id` (`space_id`,`experiment_id`,`tag_key_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='annotate_record';