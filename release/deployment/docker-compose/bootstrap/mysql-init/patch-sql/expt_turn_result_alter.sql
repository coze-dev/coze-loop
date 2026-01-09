ALTER TABLE `expt_turn_result`
    ADD COLUMN `weighted_score` decimal(10, 4) DEFAULT NULL COMMENT '加权汇总得分' AFTER `err_msg`;
