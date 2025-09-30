-- Copyright (c) 2025 coze-dev Authors
-- SPDX-License-Identifier: Apache-2.0

-- Create database if not exists
CREATE DATABASE IF NOT EXISTS cozeloop_evaluation;

-- Create expt_turn_result_filter_local table for docker environment
CREATE TABLE IF NOT EXISTS cozeloop_evaluation.expt_turn_result_filter_local
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
    INDEX inv_eval_target_data_actual_output eval_target_data TYPE inverted GRANULARITY 1
)
ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/{database}/{table}', '{replica}')
PARTITION BY created_date
ORDER BY (expt_id, cityHash64(item_id), turn_id)
SAMPLE BY cityHash64(item_id)
SETTINGS index_granularity = 8192;