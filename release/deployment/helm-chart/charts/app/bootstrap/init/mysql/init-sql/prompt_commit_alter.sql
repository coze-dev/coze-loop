ALTER TABLE `prompt_commit` ADD COLUMN IF NOT EXISTS `ext_info` text COLLATE utf8mb4_general_ci COMMENT 'Extended information field';
ALTER TABLE `prompt_commit` ADD COLUMN IF NOT EXISTS `metadata` text COLLATE utf8mb4_general_ci COMMENT 'Template metadata field';
