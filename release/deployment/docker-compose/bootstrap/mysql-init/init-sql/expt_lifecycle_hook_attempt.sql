CREATE TABLE IF NOT EXISTS `expt_lifecycle_hook_attempt`
(
    `id` bigint NOT NULL,
    `operation_id` varchar(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `attempt` int NOT NULL,
    `delivery_id` varchar(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    `lease_generation` bigint unsigned NOT NULL,
    `started_at` datetime(3) NOT NULL,
    `finished_at` datetime(3) DEFAULT NULL,
    `http_status` smallint DEFAULT NULL,
    `result_category` varchar(64) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL,
    `error_redacted` blob DEFAULT NULL,
    `log_id` varchar(128) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL,
    `late_ignored` bool NOT NULL DEFAULT false,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_operation_attempt` (`operation_id`, `attempt`),
    UNIQUE KEY `uk_delivery_id` (`delivery_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;
