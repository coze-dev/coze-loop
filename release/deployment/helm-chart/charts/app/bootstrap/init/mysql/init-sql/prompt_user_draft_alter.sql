ALTER TABLE `prompt_user_draft` ADD COLUMN `ext_info` text COLLATE utf8mb4_general_ci COMMENT 'Extended information field';
ALTER TABLE `prompt_user_draft` ADD COLUMN `metadata` text COLLATE utf8mb4_general_ci COMMENT 'Template metadata field';
ALTER TABLE `prompt_user_draft` ADD COLUMN `has_snippets` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否包含prompt片段';
ALTER TABLE `prompt_user_draft` ADD COLUMN `encrypt_messages` longtext COLLATE utf8mb4_general_ci COMMENT 'encrypt message list';