CREATE TABLE IF NOT EXISTS `observability_column_extract_config`
(
    `id`             bigint unsigned                          NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    `workspace_id`   bigint unsigned                          NOT NULL DEFAULT '0' COMMENT '空间 ID',
    `platform_type`  varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '数据来源',
    `span_list_type` varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '列表信息',
    `agent_name`     varchar(512) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT 'agent名称',
    `config`         json                                     DEFAULT NULL COMMENT '提取规则配置JSON',
    `created_at`     datetime                                 NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `created_by`     varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_at`     datetime                                 NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    `updated_by`     varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '修改人',
    `is_deleted`     tinyint(1)                               NOT NULL DEFAULT '0' COMMENT '是否删除, 0 表示未删除, 1 表示已删除',
    `deleted_at`     datetime                                          DEFAULT NULL COMMENT '删除时间',
    `deleted_by`     varchar(128) COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '删除人',
    PRIMARY KEY (`id`),
    KEY `idx_space` (`workspace_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='列提取配置';

-- 默认提取配置 (workspace_id=0 表示全局默认, '*' 表示通配)
INSERT INTO `observability_column_extract_config` (`id`, `workspace_id`, `platform_type`, `span_list_type`, `agent_name`, `config`, `created_by`, `updated_by`)
VALUES
    (1, 0, '*', 'llm_span', '', '[{"Column":"input","JSONPath":"$.messages[-1:].content"},{"Column":"output","JSONPath":"$.choices[0].message.content"}]', 'system', 'system'),
    (2, 0, 'prompt', 'root_span', '', '[{"Column":"input","JSONPath":"$.query.Content"},{"Column":"output","JSONPath":"$.choices[0].message.content"}]', 'system', 'system')
ON DUPLICATE KEY UPDATE `id` = `id`;
