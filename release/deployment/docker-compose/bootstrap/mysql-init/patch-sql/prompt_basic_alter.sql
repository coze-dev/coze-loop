ALTER TABLE `prompt_basic` ADD COLUMN `prompt_type` varchar(64) NOT NULL DEFAULT 'normal' COMMENT 'Prompt类型';
ALTER TABLE `prompt_basic` ADD KEY `idx_pid_ptype_delat` (`space_id`, `prompt_type`, `deleted_at`) USING BTREE;
ALTER TABLE `prompt_basic` ADD COLUMN `security_level` varchar(64) NOT NULL DEFAULT 'L3' COMMENT 'security level';