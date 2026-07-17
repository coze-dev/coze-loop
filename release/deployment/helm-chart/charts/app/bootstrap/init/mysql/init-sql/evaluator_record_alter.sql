ALTER TABLE `evaluator_record`
    MODIFY COLUMN `evaluator_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '评估器版本id; Inline 行写 0 哨兵(NOT NULL 不改,避免大表重建)';

ALTER TABLE `evaluator_record`
    ADD COLUMN `source_type` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '0=旧数据(语义同 Builtin) / 1=Builtin(注册评估器,含别名实例) / 2=Inline(target output 内嵌)' AFTER `evaluator_version_id`,
    ADD COLUMN `inline_key` varchar(64) NOT NULL DEFAULT '' COMMENT '仅 Inline: target output __inline_evaluators__ 的 key; 与 alias 至多一个非空' AFTER `source_type`,
    ADD COLUMN `alias` varchar(64) NOT NULL DEFAULT '' COMMENT '仅 Builtin 别名实例: 实验创建时用户输入(judge_A/judge_B)' AFTER `inline_key`,
    ADD COLUMN `target_record_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'Inline 回指来源 eval_target_record.id; Builtin 为 0' AFTER `alias`;

ALTER TABLE `evaluator_record`
    ADD COLUMN `item_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=旧数据/无版本概念; 从 expt_item_ref 同步' AFTER `item_id`;
