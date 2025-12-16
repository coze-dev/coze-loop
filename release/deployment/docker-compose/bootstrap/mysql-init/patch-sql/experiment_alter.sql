ALTER TABLE `experiment`
    ADD COLUMN `expt_template_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验模板 id' AFTER `eval_set_id`;
