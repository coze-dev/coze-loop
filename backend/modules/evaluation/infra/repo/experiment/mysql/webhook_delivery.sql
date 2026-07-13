-- Migration: add `webhook_delivery` (T2.3).
-- - `uk_delivery_id` guarantees idempotency at MySQL layer even if the app
--   layer accidentally publishes the same event twice.
-- - `idx_experiment (experiment_id, event)` speeds up per-experiment audit lookups.
-- - `idx_retry (status, last_sent_at)` fuels the retry scan loop.

CREATE TABLE IF NOT EXISTS `webhook_delivery` (
  `id`                  BIGINT UNSIGNED   NOT NULL AUTO_INCREMENT,
  `delivery_id`         VARCHAR(64)       NOT NULL COMMENT 'UUIDv4',
  `space_id`            BIGINT UNSIGNED   NOT NULL,
  `experiment_id`       BIGINT UNSIGNED   NOT NULL,
  `event`               VARCHAR(32)       NOT NULL COMMENT 'started/succeeded/failed/terminated',
  `url`                 VARCHAR(1024)     NOT NULL,
  `payload`             JSON              NULL,
  `status`              VARCHAR(32)       NOT NULL COMMENT 'pending/retrying/succeeded/failed/final_failed/rate_limited',
  `attempt_count`       INT               NOT NULL DEFAULT 0,
  `first_sent_at`       DATETIME          NULL,
  `last_sent_at`        DATETIME          NULL,
  `last_response_code`  INT               NOT NULL DEFAULT 0,
  `last_error`          VARCHAR(2048)     NOT NULL DEFAULT '',
  `internal_source`     VARCHAR(32)       NOT NULL DEFAULT '' COMMENT 'bits / empty',
  `created_at`          DATETIME          NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at`          DATETIME          NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_delivery_id` (`delivery_id`),
  KEY `idx_experiment` (`experiment_id`, `event`),
  KEY `idx_retry` (`status`, `last_sent_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='experiment webhook delivery audit';
