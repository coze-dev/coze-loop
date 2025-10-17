CREATE TABLE IF NOT EXISTS `auto_task_run` (
                               `id` bigint unsigned NOT NULL COMMENT 'TaskRun ID',
                               `workspace_id` bigint unsigned NOT NULL COMMENT '空间ID',
                               `task_id` bigint unsigned NOT NULL COMMENT 'Task ID',
                               `task_type` varchar(64) NOT NULL DEFAULT '' COMMENT 'Task类型',
                               `run_status` varchar(64) NOT NULL DEFAULT '' COMMENT 'Task Run状态',
                               `run_detail` json DEFAULT NULL COMMENT 'Task Run运行状态详情',
                               `backfill_detail` json DEFAULT NULL COMMENT '历史回溯Task Run运行状态详情',
                               `run_start_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '任务开始时间',
                               `run_end_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '任务结束时间',
                               `run_config` json DEFAULT NULL COMMENT '相关Run的配置信息',
                               `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
                               `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
                               PRIMARY KEY (`id`),
                               KEY `idx_task_id_status` (`task_id`,`run_status`),
                               KEY `idx_workspace_task` (`workspace_id`, `task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='Task Run信息';