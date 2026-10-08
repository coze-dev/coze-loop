CREATE TABLE IF NOT EXISTS `expt_lifecycle_run_item`
(
    `id` bigint unsigned NOT NULL,
    `space_id` bigint NOT NULL,
    `expt_id` bigint NOT NULL,
    `expt_run_id` bigint NOT NULL,
    `ordinal` bigint NOT NULL,
    `source_space_id` bigint NOT NULL,
    `eval_set_id` bigint NOT NULL,
    `eval_set_version_id` bigint NOT NULL DEFAULT '0',
    `item_id` bigint NOT NULL,
    `item_version_id` bigint NOT NULL DEFAULT '0',
    `admitted_at` datetime(3) DEFAULT NULL,
    `execution_manifest` mediumblob DEFAULT NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_run_item` (`space_id`, `expt_run_id`, `item_id`),
    KEY `idx_run_ordinal` (`space_id`, `expt_run_id`, `ordinal`, `id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;
