CREATE TABLE IF NOT EXISTS `expt_template_trigger`
(
    `id` bigint NOT NULL,
    `binding_id` varchar(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `binding_version` bigint unsigned NOT NULL,
    `instance_id` varchar(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `space_id` bigint NOT NULL,
    `template_id` bigint NOT NULL,
    `expt_id` bigint NOT NULL,
    `expt_run_id` bigint NOT NULL,
    `status` varchar(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending',
    `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_binding_instance` (`binding_id`, `binding_version`, `instance_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;
