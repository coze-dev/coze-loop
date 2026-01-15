CREATE TABLE IF NOT EXISTS `expt_template`
(
    `id`                  bigint unsigned                                                NOT NULL COMMENT 'id',
    `space_id`            bigint unsigned                                                NOT NULL DEFAULT '0' COMMENT '空间 id',
    `name`                varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci  NOT NULL DEFAULT '' COMMENT '实验模板名称',
    `description`         varchar(1024) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '实验模板描述',

    `eval_set_id`         bigint unsigned                                                NOT NULL DEFAULT '0' COMMENT '评测集 id（模板创建后不可修改）',
    `eval_set_version_id` bigint unsigned                                                NOT NULL DEFAULT '0' COMMENT '评测集默认版本 id',
    `target_id`           bigint unsigned                                                NOT NULL DEFAULT '0' COMMENT '评估对象 id（模板创建后不可修改）',
    `target_type`         bigint unsigned                                                NOT NULL DEFAULT '0' COMMENT '评估对象类型',
    `target_version_id`   bigint unsigned                                                NOT NULL DEFAULT '0' COMMENT '评估对象默认版本 id',

    `expt_type`           int unsigned                                                   NOT NULL DEFAULT '1' COMMENT '实验类型，offline:1,online:2...',

    `template_conf`       blob COMMENT '实验模板配置，包含评估器列表、字段映射、加权配置、默认并发及调度等，json',
    `expt_info`           blob COMMENT '实验运行状态，包含创建实验数量，最后一次实验执行状态，json',

    `created_by`           varchar(128)    NOT NULL DEFAULT '0' COMMENT '创建人',
    `updated_by`           varchar(128)    NOT NULL DEFAULT '0' COMMENT '更新人',
    `created_at`           timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`           timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`           timestamp       NULL     DEFAULT NULL COMMENT '删除时间',

    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_space_id_name_deleted_at` (`space_id`, `name`, `deleted_at`),
    KEY `idx_space_id_created_by_deleted_at` (`space_id`, `created_by`, `deleted_at`),
    KEY `idx_space_id_eval_set_id_deleted_at` (`space_id`, `eval_set_id`, `deleted_at`),
    KEY `idx_space_id_target_id_deleted_at` (`space_id`, `target_id`, `deleted_at`),
    KEY `idx_space_id_expt_type_deleted_at` (`space_id`, `expt_type`, `deleted_at`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='expt_template';


