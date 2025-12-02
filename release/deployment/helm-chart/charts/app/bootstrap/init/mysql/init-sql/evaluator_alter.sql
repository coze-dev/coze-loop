ALTER TABLE evaluator
    ADD COLUMN `evaluator_info` blob COMMENT '评估器补充信息, json',
    ADD COLUMN `builtin` int unsigned NOT NULL DEFAULT '2' COMMENT '是否预置，1:是；2:否',
    ADD COLUMN `box_type` int unsigned NOT NULL DEFAULT '1' COMMENT '黑白盒类型，1:白盒；2:黑盒',
    ADD COLUMN `builtin_visible_version` varchar(128) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '预置评估器最新可见版本号';