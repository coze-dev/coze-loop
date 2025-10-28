CREATE TABLE `evaluator_tag` (
                                 `id` bigint unsigned NOT NULL COMMENT 'idgen id',
                                 `source_id` bigint unsigned NOT NULL COMMENT '资源id',
                                 `tag_type` int unsigned NOT NULL COMMENT 'tag类型，1:评估器；2:模板',
                                 `tag_key` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '0' COMMENT 'tag键',
                                 `tag_value` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '0' COMMENT 'tag值',
                                 `created_by` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '0' COMMENT '创建人',
                                 `updated_by` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '0' COMMENT '更新人',
                                 `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                                 `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                                 `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
                                 PRIMARY KEY (`id`),
                                 KEY `idx_source_id` (`source_id`),
                                 KEY `idx_tag_type_tag_key_tag_value` (`tag_type`,`tag_key`,`tag_value`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='NDB_SHARE_TABLE;评估器tag';