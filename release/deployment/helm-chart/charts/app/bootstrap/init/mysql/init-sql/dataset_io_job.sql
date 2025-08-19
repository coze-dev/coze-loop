CREATE TABLE IF NOT EXISTS `dataset_io_job`
(
    `id`                 bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID',
    `app_id`             int unsigned    NOT NULL DEFAULT '0' COMMENT '应用 ID',
    `space_id`           bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 ID',
    `dataset_id`         bigint unsigned NOT NULL DEFAULT '0' COMMENT '数据集 ID',
    `job_type`           varchar(128)    NOT NULL DEFAULT '' COMMENT '任务类型',
    `source_file`        json                     DEFAULT NULL COMMENT '源文件信息',
    `source_dataset`     json                     DEFAULT NULL COMMENT '源数据集信息',
    `target_file`        json                     DEFAULT NULL COMMENT '目标文件信息',
    `target_dataset`     json                     DEFAULT NULL COMMENT '目标数据集信息',
    `field_mappings`     json                     DEFAULT NULL COMMENT '字段映射',
    `option`             json                     DEFAULT NULL COMMENT '任务选项',
    `status`             varchar(128)    NOT NULL DEFAULT '' COMMENT '状态',
    `progress_total`     bigint unsigned NOT NULL DEFAULT '0' COMMENT '总数',
    `progress_processed` bigint unsigned NOT NULL DEFAULT '0' COMMENT '已处理的数量',
    `progress_added`     bigint unsigned NOT NULL DEFAULT '0' COMMENT '已写入的数量',
    `sub_progresses`     json                     DEFAULT NULL COMMENT '进度信息',
    `errors`             json                     DEFAULT NULL COMMENT '错误信息',
    `created_by`         varchar(128)    NOT NULL DEFAULT '' COMMENT '创建人',
    `created_at`         timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_by`         varchar(128)    NOT NULL DEFAULT '' COMMENT '修改人',
    `updated_at`         timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '修改时间',
    `started_at`         timestamp       NULL     DEFAULT NULL COMMENT '开始时间',
    `ended_at`           timestamp       NULL     DEFAULT NULL COMMENT '结束时间',
    PRIMARY KEY (`id`),
    KEY `idx_space_dataset` (`space_id`, `dataset_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='数据集导入导出任务';