-- model_key: 空间内唯一的语义化模型 key(slug, ≤128, 一经设置不可修改)。
-- 存量数据 model_key 保持 NULL, UNIQUE 索引对 NULL 允许多值并存, 不阻塞老模型。
ALTER TABLE `model` ADD COLUMN `model_key` varchar(128) DEFAULT NULL COMMENT '空间内唯一的语义化模型 key';
ALTER TABLE `model` ADD UNIQUE KEY `uk_space_model_key` (`workspace_id`, `model_key`) USING BTREE COMMENT 'workspace_id + model_key 唯一索引, NULL 多值兼容';
