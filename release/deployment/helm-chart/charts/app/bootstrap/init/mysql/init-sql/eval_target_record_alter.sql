ALTER TABLE `eval_target_record` ADD COLUMN `ext` json DEFAULT NULL COMMENT 'ext';

ALTER TABLE `eval_target_record`
    ADD COLUMN `item_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=旧数据/无版本概念; 从 expt_item_ref 同步' AFTER `item_id`;
