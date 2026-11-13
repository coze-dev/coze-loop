-- Copyright (c) 2025 coze-dev Authors
-- SPDX-License-Identifier: Apache-2.0
--
-- 中心调度 run 级元数据投影表。
-- 关键约束：
--   1) (space_id, expt_id, run_id) 唯一：重复进入 Pending 只更新不产生第二行；
--   2) idx_scope_status_priority 支撑全局扫描 owner_scope + 活跃状态 + priority/created_at；
--   3) schedule_mode 与 priority 真实落库，不是内存/临时存储。

CREATE TABLE IF NOT EXISTS `expt_run_schedule` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `space_id` bigint(20) unsigned NOT NULL COMMENT '空间 id',
  `expt_id` bigint(20) unsigned NOT NULL COMMENT '实验 id',
  `run_id` bigint(20) unsigned NOT NULL COMMENT '运行 id',
  `schedule_mode` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '调度模式 0=legacy 1=central',
  `priority` int(11) NOT NULL DEFAULT '0' COMMENT '优先级，越大越优先',
  `owner_scope` varchar(255) CHARACTER SET utf8mb4 NOT NULL DEFAULT '' COMMENT '所有权隔离键 env+lane+region+psm',
  `target_concurrency` bigint(20) NOT NULL DEFAULT '0' COMMENT '目标 item 并发',
  `status` int(11) unsigned NOT NULL DEFAULT '1' COMMENT '1=pending 2=active 3=draining 4=terminal',
  `admission_token` varchar(128) NOT NULL DEFAULT '' COMMENT '旧 200 准入 token',
  `ledger_generation` bigint(20) NOT NULL DEFAULT '0' COMMENT '账本代次投影',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_space_expt_run` (`space_id`,`expt_id`,`run_id`),
  KEY `idx_scope_status_priority` (`owner_scope`,`status`,`priority`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='实验 run 中心调度元数据投影';
