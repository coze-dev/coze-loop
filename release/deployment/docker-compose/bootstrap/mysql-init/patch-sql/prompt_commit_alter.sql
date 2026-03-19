ALTER TABLE `prompt_commit` ADD COLUMN `ext_info` text COLLATE utf8mb4_general_ci COMMENT 'Extended information field';
ALTER TABLE `prompt_commit` ADD COLUMN `metadata` text COLLATE utf8mb4_general_ci COMMENT 'Template metadata field';
ALTER TABLE `prompt_commit` ADD COLUMN `has_snippets` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否包含prompt片段';
ALTER TABLE `prompt_commit` ADD COLUMN `mcp_config` text COLLATE utf8mb4_general_ci COMMENT 'mcp config info';