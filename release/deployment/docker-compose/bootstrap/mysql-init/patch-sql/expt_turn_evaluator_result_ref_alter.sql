ALTER TABLE `expt_turn_evaluator_result_ref`
    MODIFY COLUMN `evaluator_version_id` bigint unsigned NOT NULL COMMENT '评估器版本 id; Inline 行写 0 哨兵';

ALTER TABLE `expt_turn_evaluator_result_ref`
    ADD COLUMN `source_type` tinyint unsigned NOT NULL DEFAULT '0' COMMENT '0=旧数据(语义同 Builtin) / 1=Builtin / 2=Inline' AFTER `evaluator_version_id`,
    ADD COLUMN `inline_key` varchar(64) NOT NULL DEFAULT '' COMMENT '仅 Inline: target output __inline_evaluators__ 的 key' AFTER `source_type`,
    ADD COLUMN `alias` varchar(64) NOT NULL DEFAULT '' COMMENT '仅 Builtin 别名实例; 与 inline_key 至多一个非空' AFTER `inline_key`;

-- 老唯一键 (space_id, expt_turn_result_id, evaluator_version_id) 无法区分 alias/inline 多实例, 会被 ON DUPLICATE KEY 覆盖; 先 DROP 再建新 5 列唯一键
ALTER TABLE `expt_turn_evaluator_result_ref`
    DROP KEY `uk_space_expt_turn_result_evaluator`;

ALTER TABLE `expt_turn_evaluator_result_ref`
    ADD UNIQUE KEY `uniq_expt_turn_evaluator_result_inline_alias` (`expt_id`, `evaluator_version_id`, `expt_turn_result_id`, `inline_key`, `alias`);
