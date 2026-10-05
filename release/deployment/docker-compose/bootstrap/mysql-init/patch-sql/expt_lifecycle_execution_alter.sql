ALTER TABLE `expt_lifecycle_run` ADD COLUMN `execution_initialized` bool NOT NULL DEFAULT false;
ALTER TABLE `expt_lifecycle_run_item` ADD COLUMN `execution_manifest` mediumblob DEFAULT NULL;
