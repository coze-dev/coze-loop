-- Copyright (c) 2025 coze-dev Authors
-- SPDX-License-Identifier: Apache-2.0

-- Create expt_turn_result_filter table for docker environment
CREATE TABLE IF NOT EXISTS expt_turn_result_filter
(
    `space_id` String,
    `expt_id` String,
    `item_id` String,
    `item_idx` Int32,
    `turn_id` String,
    `status` Int32,
    `eval_target_data` Map(String, String),
    `evaluator_score` Map(String, Float64),
    `annotation_float` Map(String, Float64),
    `annotation_bool` Map(String, Int8),
    `annotation_string` Map(String, String),
    `evaluator_score_corrected` Int32,
    `eval_set_version_id` String,
    `created_date` Date,
    `created_at` DateTime,
    `updated_at` DateTime,
    `eval_target_metrics` Map(String, Int64),
    `evaluator_weighted_score` Float64,
    INDEX idx_space_id space_id TYPE bloom_filter() GRANULARITY 1,
    INDEX idx_expt_id expt_id TYPE bloom_filter() GRANULARITY 1,
    INDEX idx_item_id item_id TYPE bloom_filter() GRANULARITY 1,
    INDEX idx_turn_id turn_id TYPE bloom_filter() GRANULARITY 1
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY created_date
ORDER BY (expt_id, item_id, turn_id)
SETTINGS index_granularity = 8192;
ALTER TABLE expt_turn_result_filter
ADD COLUMN IF NOT EXISTS `eval_target_metrics` Map(String, Int64)
AFTER `updated_at`;
ALTER TABLE expt_turn_result_filter
ADD COLUMN IF NOT EXISTS `evaluator_weighted_score` Float64
AFTER `eval_target_metrics`;
