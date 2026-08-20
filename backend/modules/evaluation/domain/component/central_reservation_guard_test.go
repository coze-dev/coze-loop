// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopCentralReservationGuard_ConfirmRunningFailsClosed(t *testing.T) {
	t.Parallel()

	guard := NewNoopCentralReservationGuard()

	// 关键语义：无账本环境下一律拒绝执行。
	// 放行会让 enforce 实验在零额度约束下跑（静默、事后才发现）；拒绝则可见（实验不动会被察觉）。
	ok, err := guard.ConfirmRunning(context.Background(), "fornax_cn_prod", 1, 2)
	require.NoError(t, err)
	assert.False(t, ok, "noop guard 必须 fail-closed，不得放行 enforce item")
}

func TestNoopCentralReservationGuard_ReleaseNeverErrors(t *testing.T) {
	t.Parallel()

	guard := NewNoopCentralReservationGuard()

	// 终态收口路径不应因为额度模块缺席而失败
	require.NoError(t, guard.Release(context.Background(), "fornax_cn_prod", 1, 2, "item success"))
	require.NoError(t, guard.Release(context.Background(), "fornax_cn_prod", 1, 2, "duplicate terminal event"))
}

func TestNoopCentralSchedulerScopeOwner_AlwaysOwns(t *testing.T) {
	t.Parallel()

	owner := NewNoopCentralSchedulerScopeOwner()

	// 与 Guard 的 noop 取 fail-closed 相反，本实现刻意放行：
	// 单环境部署不存在"别的环境"，拒绝执行只会让所有 enforce item 永久卡住 —— 那是自造故障。
	owned, err := owner.OwnsSchedulerScope(context.Background(), "fornax_cn_prod")
	require.NoError(t, err)
	assert.True(t, owned)

	// 空 Scope 也放行：本 port 只回答归属，"Scope 是否合法"由上游 fail-closed 判定，
	// 两处都拒绝会让职责重叠、排查时分不清是哪一层挡的。
	owned, err = owner.OwnsSchedulerScope(context.Background(), "")
	require.NoError(t, err)
	assert.True(t, owned)
}

func TestNoopCentralAdmissionPolicy_AlwaysAllows(t *testing.T) {
	t.Parallel()

	policy := NewNoopCentralAdmissionPolicy()

	// noop 取放行而非拒绝：本 policy 的职责是"在已申报向量的实验里再筛一遍"，
	// 缺省不筛等于保持引入本闸之前的语义（trigger 判据单独生效）。
	// 若取拒绝，开源部署一旦出现 EvalX trigger 就再也建不出 enforce 实验。
	allowed, err := policy.AllowCentralScheduling(context.Background(), CentralAdmissionSubject{
		SpaceID:    123,
		TargetType: "sandbox_agent",
		TargetID:   456,
	})
	require.NoError(t, err)
	assert.True(t, allowed)

	// 零值 subject（skip-target 实验：无评测对象类型与 ID）同样放行。
	allowed, err = policy.AllowCentralScheduling(context.Background(), CentralAdmissionSubject{})
	require.NoError(t, err)
	assert.True(t, allowed)
}
