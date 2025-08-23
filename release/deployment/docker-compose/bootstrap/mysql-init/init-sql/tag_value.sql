CREATE TABLE IF NOT EXISTS `tag_value` (
    `id` bigint unsigned NOT NULL COMMENT '主键id',
    `app_id` int NOT NULL DEFAULT '0' COMMENT 'application id',
    `space_id` bigint unsigned NOT NULL COMMENT '归属space id,做分片键',
    `tag_key_id` bigint unsigned NOT NULL COMMENT 'tag id，唯一标识一个标签',
    `tag_value_id` bigint unsigned NOT NULL COMMENT 'tag value id，唯一标识一个标签',
    `tag_value_name` varchar(255) NOT NULL COMMENT 'tag value名称',
    `description` varchar(2000) DEFAULT NULL COMMENT 'tag value描述',
    `parent_value_id` bigint unsigned NOT NULL COMMENT '级联标签场景,上层tag value id',
    `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间',
    `version_num` int NOT NULL DEFAULT '0' COMMENT 'tag自增版本',
    `status` varchar(32) NOT NULL DEFAULT '' COMMENT '状态,active,inactive,deprecated',
    `created_by` varchar(64) DEFAULT NULL COMMENT '创建者',
    `updated_by` varchar(64) DEFAULT NULL COMMENT '更新者',
    PRIMARY KEY (`id`),
    KEY `idx_space_id_tag_key_id_version_num` (`space_id`,`tag_key_id`,`version_num`),
    KEY `idx_space_id_tag_value_id_version_num` (`space_id`,`tag_value_id`,`version_num`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='tag value元数据表'