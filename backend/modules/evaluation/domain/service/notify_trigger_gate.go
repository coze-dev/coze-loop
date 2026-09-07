// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// isFeishuNotifySuppressedByTrigger 判断该实验是否因触发来源而整体抑制飞书通知。
//
// 当前规则：trigger_type == evalx 的实验一律不发实验生命周期相关的飞书卡片
// （提交/终态卡、沙箱 agent 进度卡与单行失败卡）。EvalX 是内部平台，实验由它批量发起、
// 结果也在它自己的面板消费，逐个实验给创建者推飞书卡只会造成打扰。
//
// 判据取落库后的 expt.TriggerType 而非请求字段：写入路径会对调用方申报的 trigger 做裁决
// （enforceSchedulingPrivilege 会把未获授权的自称 evalx 降级成 manual），因此读落库值
// 既不会被伪造绕过、也不会误伤普通实验。
//
// 复用 entity.ShouldEnforceByTrigger 而不是自己再比一次字面量：它已经是本仓库
// "这个 trigger 是不是 evalx" 的唯一判据（含大小写/空白容忍，取值由
// TestExptTriggerTypeEvalxValue 守住与 IDL 常量一致），另起一份会漂移。
//
// 不覆盖洞察分析完成卡（insight analysis）：那是人在页面上主动点分析后的回执，
// 不属于实验生命周期通知，挡掉会让操作没有反馈。
func isFeishuNotifySuppressedByTrigger(expt *entity.Experiment) bool {
	if expt == nil {
		return false
	}
	return entity.ShouldEnforceByTrigger(expt.TriggerType)
}
