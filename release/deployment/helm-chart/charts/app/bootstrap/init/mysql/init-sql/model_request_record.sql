CREATE TABLE IF NOT EXISTS `model_request_record`
(
    `id`                    bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '自增主键ID',
    `space_id`              bigint unsigned NOT NULL DEFAULT '0' COMMENT '空间id',
    `user_id`               varchar(256)    NOT NULL DEFAULT '' COMMENT 'user id',
    `usage_scene`           varchar(128)    NOT NULL DEFAULT '' COMMENT '场景',
    `usage_scene_entity_id` varchar(256)    NOT NULL DEFAULT '' COMMENT '场景实体id',
    `frame`                 varchar(128)    NOT NULL DEFAULT '' COMMENT '使用的框架，如eino',
    `protocol`              varchar(128)    NOT NULL DEFAULT '' COMMENT '使用的协议，如ark/deepseek等',
    `model_identification`  varchar(1024)   NOT NULL DEFAULT '' COMMENT '模型唯一标识',
    `model_ak`              varchar(1024)   NOT NULL DEFAULT '' COMMENT '模型的AK',
    `model_id`              varchar(256)    NOT NULL DEFAULT '' COMMENT 'model id',
    `model_name`            varchar(1024)   NOT NULL DEFAULT '' COMMENT '模型展示名称',
    `input_token`           bigint unsigned NOT NULL DEFAULT '0' COMMENT '输入token数量',
    `output_token`          bigint unsigned NOT NULL DEFAULT '0' COMMENT '输出token数量',
    `logid`                 varchar(128)    NOT NULL DEFAULT '' COMMENT 'logid',
    `error_code`            varchar(128)    NOT NULL DEFAULT '' COMMENT 'error_code',
    `error_msg`             text COLLATE utf8mb4_general_ci COMMENT 'error_msg',
    `created_at`            datetime        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`            datetime        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_space_id_create_time` (`space_id`, `created_at`) USING BTREE COMMENT 'space_id_create_time'
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_general_ci COMMENT ='模型流量记录开源表';