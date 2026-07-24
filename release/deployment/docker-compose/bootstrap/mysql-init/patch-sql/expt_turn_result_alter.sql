ALTER TABLE `expt_turn_result`
    ADD COLUMN `weighted_score` decimal(10, 4) DEFAULT NULL COMMENT '加权汇总得分' AFTER `err_msg`;

ALTER TABLE `expt_turn_result`
    ADD COLUMN `item_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=旧数据/无版本概念; turn 级筛选用; 真值源 expt_item_ref' AFTER `item_id`;
