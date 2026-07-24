ALTER TABLE `expt_evaluator_ref`
    ADD COLUMN `eval_set_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '该 binding 归属的评测集 id(反查标签); 0=老数据/单 set' AFTER `expt_id`,
    ADD COLUMN `alias` varchar(64) NOT NULL DEFAULT '' COMMENT '别名: 同 (evaluator_id, evaluator_version_id) 多实例区分(judge_A/judge_B); 默认实例为空串' AFTER `evaluator_version_id`,
    ADD COLUMN `filter` blob COMMENT '行级过滤配置快照, json: {filter_fields: [...], filter_mode: 0 None/1 Include/2 Exclude}; 仅供查询' AFTER `alias`,
    ADD COLUMN `binding_config` blob COMMENT 'binding 配置快照, json: {IngressConf, RunConf, ScoreWeight}; 仅供查询' AFTER `filter`;
