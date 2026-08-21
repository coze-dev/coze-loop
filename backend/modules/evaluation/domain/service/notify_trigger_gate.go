// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// exptTriggerTypeEvalx EvalX 平台发起实验时携带的 trigger_type，落 experiment.trigger_type 列。
//
// 这里不 import kitex_gen 的 IDL 常量：domain 层不依赖生成代码是本仓库的分层约束。
const exptTriggerTypeEvalx = "evalx"

// isFeishuNotifySuppressedByTrigger 判断该实验是否因触发来源而整体抑制飞书通知。
//
// 当前规则：trigger_type == evalx 的实验一律不发实验生命周期相关的飞书卡片
// （提交/终态卡、沙箱 agent 进度卡与单行失败卡）。EvalX 是内部平台，实验由它批量发起、
// 结果也在它自己的面板消费，逐个实验给创建者推飞书卡只会造成打扰。
//
// 判据取落库后的 expt.TriggerType 而非请求字段：写入路径会对调用方申报的 trigger 做裁决
// （未获授权的自称 evalx 会被降级），因此读落库值既不会被伪造绕过、也不会误伤普通实验。
//
// 大小写与空白容忍：trigger_type 是跨系统传递的字符串，上游拼装方式不受本仓库控制。
//
// 不覆盖洞察分析完成卡（insight analysis）：那是人在页面上主动点分析后的回执，
// 不属于实验生命周期通知，挡掉会让操作没有反馈。
func isFeishuNotifySuppressedByTrigger(expt *entity.Experiment) bool {
	if expt == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(expt.TriggerType), exptTriggerTypeEvalx)
}
