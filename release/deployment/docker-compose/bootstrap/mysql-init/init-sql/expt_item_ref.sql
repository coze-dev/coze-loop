CREATE TABLE IF NOT EXISTS `expt_item_ref`
(
    `id`                  bigint unsigned NOT NULL COMMENT 'idgen id',
    `space_id`            bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间 id',
    `expt_id`             bigint unsigned NOT NULL DEFAULT '0' COMMENT '实验 id',
    `item_id`             bigint unsigned NOT NULL DEFAULT '0' COMMENT 'dataset_item.item_id, idgen 全局唯一',
    `item_version_id`     bigint unsigned NOT NULL DEFAULT '0' COMMENT 'item 自身版本号; 0=无版本概念; 与 dataset_version 独立; 全链路真值源',
    `eval_set_id`         bigint unsigned NOT NULL DEFAULT '0' COMMENT '归属评测集标签(前端分组 / CK 分桶 / 反查; 调度不读)',
    `eval_set_version_id` bigint unsigned NOT NULL DEFAULT '0' COMMENT '调度键: 配合 item_id 定位 dataset_item_snapshot',
    `item_config`         mediumblob COMMENT 'per-item 行级配置, json: {eval_target_conf?, evaluator_conf?(含 version_id/alias/映射/动态参数/filter), turn_indexes?, ext?}; 单行执行唯一配置源',
    `order_idx`           int unsigned    NOT NULL DEFAULT '0' COMMENT '实验内执行/展示顺序',
    `created_at`          timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`          timestamp       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`          timestamp       NULL     DEFAULT NULL COMMENT '删除时间',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uniq_space_id_expt_id_item_id` (`space_id`, `expt_id`, `item_id`),
    KEY `idx_space_eval_set_version_expt` (`space_id`, `eval_set_id`, `eval_set_version_id`, `expt_id`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='实验绑定 item 及行级配置(首次调度写入, 单行执行唯一配置源)';
