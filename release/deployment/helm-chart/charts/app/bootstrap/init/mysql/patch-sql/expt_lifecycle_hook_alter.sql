ALTER TABLE `experiment` ADD COLUMN `lifecycle_hook_conf` mediumblob DEFAULT NULL COMMENT '受保护的生命周期Hook配置封套';
ALTER TABLE `expt_template` ADD COLUMN `lifecycle_hook_conf` mediumblob DEFAULT NULL COMMENT '受保护的生命周期Hook配置封套';
ALTER TABLE `expt_template` ADD COLUMN `schedule_run_binding` mediumblob DEFAULT NULL COMMENT '服务端定时执行身份绑定封套';
ALTER TABLE `expt_run_log` ADD COLUMN `lifecycle_hook_version` smallint unsigned DEFAULT NULL COMMENT 'NULL/0 legacy, 1 lifecycle managed';
ALTER TABLE `expt_lifecycle_run` ADD COLUMN `before_enabled` bool NOT NULL DEFAULT false COMMENT '创建时冻结的before启用状态';
ALTER TABLE `expt_lifecycle_run` ADD COLUMN `after_enabled` bool NOT NULL DEFAULT false COMMENT '创建时冻结的after启用状态';
ALTER TABLE `expt_lifecycle_hook_run` ADD COLUMN `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3);
