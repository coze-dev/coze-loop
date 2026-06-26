CREATE TABLE IF NOT EXISTS `evaluator_record`
(
    `id`                   bigint unsigned  NOT NULL COMMENT 'idgen id',
    `space_id`             bigint unsigned  NOT NULL COMMENT '空间id',
    `evaluator_version_id` bigint unsigned  NOT NULL DEFAULT '0' COMMENT '评估器版本id; Inline 行写 0 哨兵(NOT NULL 不改,避免大表重建)',
    `source_type`          tinyint unsigned NOT NULL DEFAULT '0' COMMENT '0=旧数据(语义同 Builtin) / 1=Builtin(注册评估器,含别名实例) / 2=Inline(target output 内嵌)',
    `inline_key`           varchar(64)      NOT NULL DEFAULT '' COMMENT '仅 Inline: target output __inline_evaluators__ 的 key; 与 alias 至多一个非空',
    `alias`                varchar(64)      NOT NULL DEFAULT '' COMMENT '仅 Builtin 别名实例: 实验创建时用户输入(judge_A/judge_B)',
    `target_record_id`     bigint unsigned  NOT NULL DEFAULT '0' COMMENT 'Inline 回指来源 eval_target_record.id; Builtin 为 0',
    `experiment_id`        bigint unsigned           DEFAULT NULL COMMENT '实验id',
    `experiment_run_id`    bigint unsigned  NOT NULL COMMENT '实验执行id',
    `item_id`              bigint unsigned  NOT NULL COMMENT '评估集行id',
    `item_version_id`      bigint unsigned  NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=旧数据/无版本概念; 从 expt_item_ref 同步',
    `turn_id`              bigint unsigned  NOT NULL DEFAULT '0' COMMENT '评估集行轮次id',
    `log_id`               varchar(255)              DEFAULT NULL COMMENT 'log id',
    `trace_id`             varchar(255)     NOT NULL COMMENT 'trace id',
    `score`                decimal(10, 4)            DEFAULT NULL COMMENT '得分',
    `status`               int              NOT NULL COMMENT '执行状态',
    `created_at`           timestamp        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`           timestamp        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`           timestamp        NULL     DEFAULT NULL COMMENT '删除时间',
    `input_data`           mediumblob COMMENT '输入, json',
    `output_data`          mediumblob COMMENT '执行结果, json',
    `created_by`           varchar(128)     NOT NULL DEFAULT '0' COMMENT '创建人',
    `updated_by`           varchar(128)     NOT NULL DEFAULT '0' COMMENT '更新人',
    `ext`                  mediumblob COMMENT '补充信息, json',
    PRIMARY KEY (`id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='NDB_SHARE_TABLE;评估器执行结果';
