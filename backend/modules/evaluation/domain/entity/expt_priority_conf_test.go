// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExptPriorityWhiteList_NilAndDefaultDenyAll(t *testing.T) {
	// nil 与缺省配置都必须拒绝 —— 这是 fail-closed 方向：
	// 读不到配置时让所有人退回缺省优先级（可见、无损），而不是静默放开插队。
	var nilList *ExptPriorityWhiteList
	assert.False(t, nilList.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "zhangsan@bytedance.com", CallerPSM: "a.b.c"}),
		"nil 白名单必须拒绝")

	assert.False(t, DefaultExptPriorityWhiteList().AllowSpecifyPriority(
		ExptPrioritySubject{UserEmail: "zhangsan@bytedance.com", CallerPSM: "a.b.c"}),
		"缺省白名单（读取失败时的兜底）必须拒绝")
}

func TestExptPriorityWhiteList_AllowAll(t *testing.T) {
	w := &ExptPriorityWhiteList{AllowAll: true}
	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{}), "allow_all 对空 subject 也放行")
	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "nobody@bytedance.com"}))
}

func TestExptPriorityWhiteList_ThreeDimensionsAreOR(t *testing.T) {
	// 三个维度取 OR 是刻意的：CallerPSMs 服务的是系统调用方（没有自然人 user），
	// SpaceIDs 服务的是"管理员私有空间"。取 AND 会让这两条永远走不通。
	w := &ExptPriorityWhiteList{
		UserEmails: []string{"admin@bytedance.com"},
		SpaceIDs:   []string{"222"},
		CallerPSMs: []string{"stone.cozeloop.evalx"},
	}

	t.Run("只命中 user", func(t *testing.T) {
		assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "admin@bytedance.com", SpaceID: 999, CallerPSM: "other.psm"}))
	})
	t.Run("只命中 space", func(t *testing.T) {
		assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "nobody@bytedance.com", SpaceID: 222, CallerPSM: "other.psm"}))
	})
	t.Run("只命中 caller psm", func(t *testing.T) {
		assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "nobody@bytedance.com", SpaceID: 999, CallerPSM: "stone.cozeloop.evalx"}))
	})
	t.Run("三个都不命中", func(t *testing.T) {
		assert.False(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "nobody@bytedance.com", SpaceID: 999, CallerPSM: "other.psm"}))
	})
}

// TestExptPriorityWhiteList_SpaceIDZeroNeverMatches spaceID=0 表示"无空间上下文"，
// 绝不能因为运维在 space_ids 里误填 0 就把所有无空间请求放行。
func TestExptPriorityWhiteList_SpaceIDZeroNeverMatches(t *testing.T) {
	w := &ExptPriorityWhiteList{SpaceIDs: []string{"0"}}
	assert.False(t, w.AllowSpecifyPriority(ExptPrioritySubject{SpaceID: 0}),
		"空 spaceID 不得匹配，即使名单里误填了 \"0\"")
}

func TestExptPriorityWhiteList_UserEmailMatching(t *testing.T) {
	w := &ExptPriorityWhiteList{UserEmails: []string{"a@bytedance.com", "b@bytedance.com"}}

	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "a@bytedance.com"}))
	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: " B@ByteDance.com "}), "容忍首尾空白与大小写（邮箱大小写不敏感，名单由人手写）")

	// 解析不出来一律不放行 —— 不把"读不到身份"当成有权限。
	assert.False(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: ""}), "空邮箱不得放行 —— 拿不到身份时不把\"读不到\"当成有权限")
	assert.False(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "   "}), "纯空白邮箱不得放行")
	assert.False(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "stranger@bytedance.com"}), "不在名单内")
}

func TestExptPriorityWhiteList_CallerPSMMatching(t *testing.T) {
	w := &ExptPriorityWhiteList{CallerPSMs: []string{"stone.cozeloop.evalx", " stone.cozeloop.foo "}}

	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{CallerPSM: "stone.cozeloop.evalx"}))
	// 忽略大小写与空白：PSM 由人手写进 TCC，笔误的后果是静默不放行（配了却不生效），
	// 而两个字符串肉眼几乎一样，极难反推。
	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{CallerPSM: "Stone.CozeLoop.EvalX"}), "忽略大小写")
	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{CallerPSM: "stone.cozeloop.foo"}), "忽略名单侧的首尾空白")
	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{CallerPSM: "  stone.cozeloop.evalx  "}), "忽略入参侧的首尾空白")

	// 空 caller 一律不匹配：HTTP 入口等非 RPC 直连场景 caller 为空，
	// 那种情况该走 user 维度，不能因为"没有 caller"就放行。
	assert.False(t, w.AllowSpecifyPriority(ExptPrioritySubject{CallerPSM: ""}), "空 caller 不得放行")
	assert.False(t, w.AllowSpecifyPriority(ExptPrioritySubject{CallerPSM: "   "}), "纯空白 caller 不得放行")
	assert.False(t, w.AllowSpecifyPriority(ExptPrioritySubject{CallerPSM: "evil.psm"}))
}

// TestExptPriorityWhiteList_EmptyListsDoNotMatch 各维度列表为空时不得意外放行。
// 这道回归钉住的是"空 slice 被当成通配"这类容易写错的实现。
func TestExptPriorityWhiteList_EmptyListsDoNotMatch(t *testing.T) {
	w := &ExptPriorityWhiteList{}
	assert.False(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "admin@bytedance.com", SpaceID: 222, CallerPSM: "any.psm"}))
}

// ---- ExptTriggerTrustConf ----

// TestExptTriggerTrustConf_DefaultDoesNotTighten 缺省**放行**，方向与 priority 白名单刻意相反。
//
// 理由不对称：priority 配不到 → 大家退回缺省优先级，可见且无损；
// trigger 若配不到就一律拒绝 → 全部 EvalX 实验静默退回 legacy，中心调度突然没有候选，
// 现象是"实验都在跑但一个都不受额度管控"，比"配了没生效"隐蔽得多。
func TestExptTriggerTrustConf_DefaultDoesNotTighten(t *testing.T) {
	var nilConf *ExptTriggerTrustConf
	assert.True(t, nilConf.TrustEvalxTrigger("anything"), "nil 配置不得额外收紧")
	assert.True(t, nilConf.TrustEvalxTrigger(""), "nil 配置下空 caller 也放行")

	assert.True(t, DefaultExptTriggerTrustConf().TrustEvalxTrigger("anything"),
		"缺省配置（读取失败兜底）不得额外收紧")
}

// TestExptTriggerTrustConf_DisabledIgnoresList Enabled=false 时名单不生效。
//
// 用独立开关而不是"名单非空即启用"：后者会让"读取失败返回空名单"与"运维故意留空"
// 行为一致，而这两种情况的正确行为恰好相反。
func TestExptTriggerTrustConf_DisabledIgnoresList(t *testing.T) {
	c := &ExptTriggerTrustConf{Enabled: false, EvalxCallerPSMs: []string{"only.this.psm"}}
	assert.True(t, c.TrustEvalxTrigger("some.other.psm"), "未启用时名单不生效")
}

func TestExptTriggerTrustConf_EnabledChecksCaller(t *testing.T) {
	c := &ExptTriggerTrustConf{Enabled: true, EvalxCallerPSMs: []string{"stone.cozeloop.evalx", " stone.cozeloop.foo "}}

	assert.True(t, c.TrustEvalxTrigger("stone.cozeloop.evalx"))
	assert.True(t, c.TrustEvalxTrigger("Stone.CozeLoop.EvalX"), "忽略大小写")
	assert.True(t, c.TrustEvalxTrigger("stone.cozeloop.foo"), "忽略名单侧空白")
	assert.True(t, c.TrustEvalxTrigger("  stone.cozeloop.evalx  "), "忽略入参侧空白")

	assert.False(t, c.TrustEvalxTrigger("evil.psm"), "名单外的调用方不得自称 evalx")
	// 启用校验后拿不到 caller 即不可采信：我们信的是框架填充的身份，没有身份就没有可信来源。
	assert.False(t, c.TrustEvalxTrigger(""), "启用后空 caller 不可采信")
	assert.False(t, c.TrustEvalxTrigger("   "), "启用后纯空白 caller 不可采信")
}

// TestExptTriggerTrustConf_EnabledWithEmptyListDeniesAll 启用但名单为空 = 谁都不可信。
// 这是显式配置的结果（Enabled 被手动打开），与"读取失败"不同，因此拒绝是对的。
func TestExptTriggerTrustConf_EnabledWithEmptyListDeniesAll(t *testing.T) {
	c := &ExptTriggerTrustConf{Enabled: true}
	assert.False(t, c.TrustEvalxTrigger("any.psm"))
}

// TestExptPriorityWhiteList_UnmarshalFullConfig 端到端解一份完整配置，
// 确认字段名与运维实际会写的 JSON 一致（字段名写错的后果是静默不生效）。
func TestExptPriorityWhiteList_UnmarshalFullConfig(t *testing.T) {
	raw := `{
		"user_emails": ["admin@bytedance.com"],
		"space_ids": ["7533128632407949313"],
		"caller_psms": ["stone.cozeloop.evalx"],
		"allow_all": false
	}`

	var w ExptPriorityWhiteList
	require.NoError(t, json.Unmarshal([]byte(raw), &w))

	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "admin@bytedance.com"}))
	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{SpaceID: 7533128632407949313}),
		"19 位雪花 ID 从字符串配置解出后必须精确匹配")
	assert.True(t, w.AllowSpecifyPriority(ExptPrioritySubject{CallerPSM: "stone.cozeloop.evalx"}))
	assert.False(t, w.AllowSpecifyPriority(ExptPrioritySubject{UserEmail: "other@bytedance.com", SpaceID: 999}))
}

// TestExptTriggerTrustConf_UnmarshalFullConfig 同上，钉住 trigger 配置的字段名。
func TestExptTriggerTrustConf_UnmarshalFullConfig(t *testing.T) {
	var c ExptTriggerTrustConf
	require.NoError(t, json.Unmarshal([]byte(`{"enabled": true, "evalx_caller_psms": ["stone.cozeloop.evalx"]}`), &c))

	assert.True(t, c.TrustEvalxTrigger("stone.cozeloop.evalx"))
	assert.False(t, c.TrustEvalxTrigger("someone.else"))
}
