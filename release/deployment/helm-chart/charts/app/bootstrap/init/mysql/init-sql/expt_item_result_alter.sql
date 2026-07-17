ALTER TABLE expt_item_result ADD COLUMN `ext` blob COMMENT '补充信息';

ALTER TABLE `expt_item_result`
    ADD COLUMN `item_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=旧数据/无版本概念; 真值源 expt_item_ref' AFTER `item_id`;
