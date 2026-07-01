-- 公共模型表 model_public 加列 model_key + UNIQUE(workspace_id, model_key)。
-- 与 model_alter.sql 语义一致, 存量 NULL 不回填, 后续每模型可补填一次。
ALTER TABLE `model_public` ADD COLUMN `model_key` varchar(128) DEFAULT NULL COMMENT '空间内唯一的语义化模型 key';
ALTER TABLE `model_public` ADD UNIQUE KEY `uk_space_model_key` (`workspace_id`, `model_key`) USING BTREE COMMENT 'workspace_id + model_key 唯一索引, NULL 多值兼容';
