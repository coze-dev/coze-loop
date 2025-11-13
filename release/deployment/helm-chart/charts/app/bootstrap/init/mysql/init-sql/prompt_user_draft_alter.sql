ALTER TABLE `prompt_user_draft` ADD COLUMN `ext_info` text COLLATE utf8mb4_general_ci COMMENT 'Extended information field';
ALTER TABLE `prompt_user_draft` ADD COLUMN `metadata` text COLLATE utf8mb4_general_ci COMMENT 'Template metadata field';
