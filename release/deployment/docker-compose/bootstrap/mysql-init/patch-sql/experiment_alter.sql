ALTER TABLE `experiment`
    ADD COLUMN `expt_template_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验模板 id' AFTER `eval_set_id`;

ALTER TABLE `experiment`
    ADD INDEX `idx_space_expt_template_id_delete_at` (`space_id`, `expt_template_id`, `deleted_at`);
