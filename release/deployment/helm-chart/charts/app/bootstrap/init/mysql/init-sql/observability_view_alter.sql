ALTER TABLE observability_view ADD COLUMN `scope` int unsigned NOT NULL DEFAULT '1' COMMENT '视图场景: 1-trace_list, 2-trace_detail_tree, 3-trace_detail_chat';
