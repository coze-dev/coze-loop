ALTER TABLE expt_item_result ADD COLUMN `ext` blob COMMENT '补充信息';

ALTER TABLE `expt_item_result`
    ADD COLUMN `item_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=旧数据/无版本概念; 真值源 expt_item_ref' AFTER `item_id`;

ALTER TABLE `expt_item_result`
    ADD COLUMN `backflow_status` int unsigned NOT NULL DEFAULT '0' COMMENT '回流段状态; 0=不适用(未接入回流的历史实验)' AFTER `status`,
    ADD INDEX `idx_expt_backflow_status` (`space_id`, `expt_id`, `backflow_status`);
