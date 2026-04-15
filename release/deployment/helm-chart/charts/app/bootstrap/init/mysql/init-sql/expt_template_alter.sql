ALTER TABLE `expt_template`
    ADD COLUMN `cron_activate` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否开启定时触发' AFTER `expt_type`;
