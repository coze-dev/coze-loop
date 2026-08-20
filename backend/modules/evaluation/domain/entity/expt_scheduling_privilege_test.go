// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExptSchedulingPrivilegeWhiteList_NilAndDefaultDenyAll(t *testing.T) {
	// nil 与缺省配置都必须拒绝 —— 这是 fail-closed 方向：
	// 读不到配置时让所有人退回缺省优先级（可见、无损），而不是静默放开插队。
	var nilList *ExptSchedulingPrivilegeWhiteList
	assert.False(t, nilList.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "zhangsan@bytedance.com", CallerPSM: "a.b.c"}),
		"nil 白名单必须拒绝")

	assert.False(t, DefaultExptSchedulingPrivilegeWhiteList().AllowSchedulingPrivilege(
		ExptSchedulingPrivilegeSubject{UserEmail: "zhangsan@bytedance.com", CallerPSM: "a.b.c"}),
		"缺省白名单（读取失败时的兜底）必须拒绝")
}

func TestExptSchedulingPrivilegeWhiteList_AllowAll(t *testing.T) {
	w := &ExptSchedulingPrivilegeWhiteList{AllowAll: true}
	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{}), "allow_all 对空 subject 也放行")
	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "nobody@bytedance.com"}))
}

func TestExptSchedulingPrivilegeWhiteList_ThreeDimensionsAreOR(t *testing.T) {
	// 三个维度取 OR 是刻意的：CallerPSMs 服务的是系统调用方（没有自然人 user），
	// SpaceIDs 服务的是"管理员私有空间"。取 AND 会让这两条永远走不通。
	w := &ExptSchedulingPrivilegeWhiteList{
		UserEmails: []string{"admin@bytedance.com"},
		SpaceIDs:   []string{"222"},
		CallerPSMs: []string{"stone.cozeloop.evalx"},
	}

	t.Run("只命中 user", func(t *testing.T) {
		assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "admin@bytedance.com", SpaceID: 999, CallerPSM: "other.psm"}))
	})
	t.Run("只命中 space", func(t *testing.T) {
		assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "nobody@bytedance.com", SpaceID: 222, CallerPSM: "other.psm"}))
	})
	t.Run("只命中 caller psm", func(t *testing.T) {
		assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "nobody@bytedance.com", SpaceID: 999, CallerPSM: "stone.cozeloop.evalx"}))
	})
	t.Run("三个都不命中", func(t *testing.T) {
		assert.False(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "nobody@bytedance.com", SpaceID: 999, CallerPSM: "other.psm"}))
	})
}

// TestExptSchedulingPrivilegeWhiteList_SpaceIDZeroNeverMatches spaceID=0 表示"无空间上下文"，
// 绝不能因为运维在 space_ids 里误填 0 就把所有无空间请求放行。
func TestExptSchedulingPrivilegeWhiteList_SpaceIDZeroNeverMatches(t *testing.T) {
	w := &ExptSchedulingPrivilegeWhiteList{SpaceIDs: []string{"0"}}
	assert.False(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{SpaceID: 0}),
		"空 spaceID 不得匹配，即使名单里误填了 \"0\"")
}

func TestExptSchedulingPrivilegeWhiteList_UserEmailMatching(t *testing.T) {
	w := &ExptSchedulingPrivilegeWhiteList{UserEmails: []string{"a@bytedance.com", "b@bytedance.com"}}

	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "a@bytedance.com"}))
	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: " B@ByteDance.com "}), "容忍首尾空白与大小写（邮箱大小写不敏感，名单由人手写）")

	// 解析不出来一律不放行 —— 不把"读不到身份"当成有权限。
	assert.False(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: ""}), "空邮箱不得放行 —— 拿不到身份时不把\"读不到\"当成有权限")
	assert.False(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "   "}), "纯空白邮箱不得放行")
	assert.False(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "stranger@bytedance.com"}), "不在名单内")
}

func TestExptSchedulingPrivilegeWhiteList_CallerPSMMatching(t *testing.T) {
	w := &ExptSchedulingPrivilegeWhiteList{CallerPSMs: []string{"stone.cozeloop.evalx", " stone.cozeloop.foo "}}

	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{CallerPSM: "stone.cozeloop.evalx"}))
	// 忽略大小写与空白：PSM 由人手写进 TCC，笔误的后果是静默不放行（配了却不生效），
	// 而两个字符串肉眼几乎一样，极难反推。
	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{CallerPSM: "Stone.CozeLoop.EvalX"}), "忽略大小写")
	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{CallerPSM: "stone.cozeloop.foo"}), "忽略名单侧的首尾空白")
	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{CallerPSM: "  stone.cozeloop.evalx  "}), "忽略入参侧的首尾空白")

	// 空 caller 一律不匹配：HTTP 入口等非 RPC 直连场景 caller 为空，
	// 那种情况该走 user 维度，不能因为"没有 caller"就放行。
	assert.False(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{CallerPSM: ""}), "空 caller 不得放行")
	assert.False(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{CallerPSM: "   "}), "纯空白 caller 不得放行")
	assert.False(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{CallerPSM: "evil.psm"}))
}

// TestExptSchedulingPrivilegeWhiteList_EmptyListsDoNotMatch 各维度列表为空时不得意外放行。
// 这道回归钉住的是"空 slice 被当成通配"这类容易写错的实现。
func TestExptSchedulingPrivilegeWhiteList_EmptyListsDoNotMatch(t *testing.T) {
	w := &ExptSchedulingPrivilegeWhiteList{}
	assert.False(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "admin@bytedance.com", SpaceID: 222, CallerPSM: "any.psm"}))
}

// TestExptSchedulingPrivilegeWhiteList_UnmarshalFullConfig 端到端解一份完整配置，
// 确认字段名与运维实际会写的 JSON 一致（字段名写错的后果是静默不生效）。
func TestExptSchedulingPrivilegeWhiteList_UnmarshalFullConfig(t *testing.T) {
	raw := `{
		"user_emails": ["admin@bytedance.com"],
		"space_ids": ["7533128632407949313"],
		"caller_psms": ["stone.cozeloop.evalx"],
		"allow_all": false
	}`

	var w ExptSchedulingPrivilegeWhiteList
	require.NoError(t, json.Unmarshal([]byte(raw), &w))

	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "admin@bytedance.com"}))
	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{SpaceID: 7533128632407949313}),
		"19 位雪花 ID 从字符串配置解出后必须精确匹配")
	assert.True(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{CallerPSM: "stone.cozeloop.evalx"}))
	assert.False(t, w.AllowSchedulingPrivilege(ExptSchedulingPrivilegeSubject{UserEmail: "other@bytedance.com", SpaceID: 999}))
}

