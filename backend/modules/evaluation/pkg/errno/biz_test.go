// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package errno

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
)

func TestNewExptZombieTimeoutErr(t *testing.T) {
	err := NewExptZombieTimeoutErr(60, 111, 222)
	ei, ok := ParseErrImpl(err)
	assert.True(t, ok)
	assert.Equal(t, ExptZombieTimeoutCode, ei.Code)
	assert.Contains(t, ei.Msg, "60s")
	assert.Contains(t, ei.Msg, "expt_id=111")
	assert.Contains(t, ei.Msg, "expt_run_id=222")

	// Round-trip serialize/deserialize keeps code + msg
	round := DeserializeErr(conv.UnsafeStringToBytes(SerializeErr(err)))
	rei, ok := ParseErrImpl(round)
	assert.True(t, ok)
	assert.Equal(t, ExptZombieTimeoutCode, rei.Code)
	assert.Equal(t, ei.Msg, rei.Msg)
}

func TestNewItemZombieTimeoutErr(t *testing.T) {
	t.Run("同步模式", func(t *testing.T) {
		err := NewItemZombieTimeoutErr(120, false)
		ei, ok := ParseErrImpl(err)
		assert.True(t, ok)
		assert.Equal(t, ItemZombieTimeoutCode, ei.Code)
		assert.True(t, strings.Contains(ei.Msg, "同步"))
		assert.Contains(t, ei.Msg, "120s")
	})

	t.Run("异步模式", func(t *testing.T) {
		err := NewItemZombieTimeoutErr(900, true)
		ei, ok := ParseErrImpl(err)
		assert.True(t, ok)
		assert.True(t, strings.Contains(ei.Msg, "异步"))
		assert.Contains(t, ei.Msg, "900s")
	})
}

func TestParseItemZombieTimeoutErr(t *testing.T) {
	t.Run("命中 ItemZombieTimeout 错误码", func(t *testing.T) {
		err := NewItemZombieTimeoutErr(60, false)
		ok, msg := ParseItemZombieTimeoutErr(err)
		assert.True(t, ok)
		assert.NotEmpty(t, msg)
	})

	t.Run("Round-trip 后仍可反解", func(t *testing.T) {
		err := NewItemZombieTimeoutErr(60, true)
		round := DeserializeErr(conv.UnsafeStringToBytes(SerializeErr(err)))
		ok, msg := ParseItemZombieTimeoutErr(round)
		assert.True(t, ok)
		assert.Contains(t, msg, "异步")
	})

	t.Run("其他 ErrImpl 类型不命中", func(t *testing.T) {
		err := NewTargetResultErr("x")
		ok, msg := ParseItemZombieTimeoutErr(err)
		assert.False(t, ok)
		assert.Empty(t, msg)
	})

	t.Run("非 ErrImpl 类型不命中", func(t *testing.T) {
		ok, msg := ParseItemZombieTimeoutErr(errors.New("plain"))
		assert.False(t, ok)
		assert.Empty(t, msg)
	})

	t.Run("nil error 不命中", func(t *testing.T) {
		ok, msg := ParseItemZombieTimeoutErr(nil)
		assert.False(t, ok)
		assert.Empty(t, msg)
	})
}

func TestParseTargetResultErr(t *testing.T) {
	t.Run("matches target result error after persistence round trip", func(t *testing.T) {
		err := DeserializeErr(conv.UnsafeStringToBytes(SerializeErr(NewTargetResultErr("failed"))))
		ok, msg := ParseTargetResultErr(err)
		assert.True(t, ok)
		assert.Equal(t, "failed", msg)
	})

	t.Run("does not match evaluator or unknown errors", func(t *testing.T) {
		ok, msg := ParseTargetResultErr(NewEvaluatorResultErr("evaluator failed"))
		assert.False(t, ok)
		assert.Empty(t, msg)

		ok, msg = ParseTargetResultErr(errors.New("plain"))
		assert.False(t, ok)
		assert.Empty(t, msg)
	})
}

func TestParseEvaluatorResultErr(t *testing.T) {
	t.Run("matches evaluator result error after persistence round trip", func(t *testing.T) {
		err := DeserializeErr(conv.UnsafeStringToBytes(SerializeErr(NewEvaluatorResultErr("failed"))))
		ok, msg := ParseEvaluatorResultErr(err)
		assert.True(t, ok)
		assert.Equal(t, "failed", msg)
	})

	t.Run("does not match target or unknown errors", func(t *testing.T) {
		ok, msg := ParseEvaluatorResultErr(NewTargetResultErr("target failed"))
		assert.False(t, ok)
		assert.Empty(t, msg)

		ok, msg = ParseEvaluatorResultErr(errors.New("plain"))
		assert.False(t, ok)
		assert.Empty(t, msg)
	})
}

func TestNewSandboxTerminatedBeforeReportErr(t *testing.T) {
	err := NewSandboxTerminatedBeforeReportErr("Failed")
	ei, ok := ParseErrImpl(err)
	assert.True(t, ok)
	assert.Equal(t, SandboxTerminatedBeforeReportCode, ei.Code)
	assert.Contains(t, ei.Msg, "Failed")

	// Round-trip serialize/deserialize keeps code + msg
	round := DeserializeErr(conv.UnsafeStringToBytes(SerializeErr(err)))
	rei, ok := ParseErrImpl(round)
	assert.True(t, ok)
	assert.Equal(t, SandboxTerminatedBeforeReportCode, rei.Code)
	assert.Equal(t, ei.Msg, rei.Msg)
}

func TestNewItemQuotaImpossibleErr(t *testing.T) {
	err := NewItemQuotaImpossibleErr("sandbox|default", 5000, 8)
	ei, ok := ParseErrImpl(err)
	assert.True(t, ok)
	assert.Equal(t, ItemQuotaImpossibleCode, ei.Code)
	// 三个数值都必须在文案里：用户要靠它判断是改申报量还是调上限。
	// 少了任何一个，错误信息就退化成"配额不够"这种无法据此行动的提示。
	assert.Contains(t, ei.Msg, "sandbox|default")
	assert.Contains(t, ei.Msg, "5000")
	assert.Contains(t, ei.Msg, "8")

	// ★ Round-trip 是这条错误码的真正契约：调度器把它序列化进 err_msg 落库，
	// 结果层再反解出来展示。任一侧编解码不一致，用户看到的就是"失败但无原因"。
	round := DeserializeErr(conv.UnsafeStringToBytes(SerializeErr(err)))
	rei, ok := ParseErrImpl(round)
	assert.True(t, ok)
	assert.Equal(t, ItemQuotaImpossibleCode, rei.Code)
	assert.Equal(t, ei.Msg, rei.Msg)
}

func TestParseItemQuotaImpossibleErr(t *testing.T) {
	t.Run("命中 ItemQuotaImpossible 错误码", func(t *testing.T) {
		ok, msg := ParseItemQuotaImpossibleErr(NewItemQuotaImpossibleErr("model|gpt5.5", 1000, 100))
		assert.True(t, ok)
		assert.Contains(t, msg, "model|gpt5.5")
	})

	t.Run("Round-trip 后仍可反解", func(t *testing.T) {
		err := NewItemQuotaImpossibleErr("sandbox|aio", 20, 2)
		round := DeserializeErr(conv.UnsafeStringToBytes(SerializeErr(err)))
		ok, msg := ParseItemQuotaImpossibleErr(round)
		assert.True(t, ok)
		assert.Contains(t, msg, "sandbox|aio")
	})

	// ★ 必须与僵尸超时互不串味：两者都写在同一个 err_msg 字段里，由结果层依次尝试反解。
	// 若彼此都能命中，item 的失败原因就会被标成另一种 —— "卡了太久"与"配置放不下"
	// 对用户的处置建议完全相反（等一等 vs 改配置）。
	t.Run("僵尸超时错误不命中", func(t *testing.T) {
		ok, msg := ParseItemQuotaImpossibleErr(NewItemZombieTimeoutErr(120, false))
		assert.False(t, ok)
		assert.Empty(t, msg)
	})

	t.Run("额度不可满足错误不被僵尸解析器命中", func(t *testing.T) {
		ok, msg := ParseItemZombieTimeoutErr(NewItemQuotaImpossibleErr("sandbox|default", 5000, 8))
		assert.False(t, ok)
		assert.Empty(t, msg)
	})

	t.Run("其他 ErrImpl 类型不命中", func(t *testing.T) {
		ok, msg := ParseItemQuotaImpossibleErr(NewTargetResultErr("x"))
		assert.False(t, ok)
		assert.Empty(t, msg)
	})

	t.Run("非 ErrImpl 类型不命中", func(t *testing.T) {
		ok, msg := ParseItemQuotaImpossibleErr(errors.New("plain"))
		assert.False(t, ok)
		assert.Empty(t, msg)
	})

	t.Run("nil error 不命中", func(t *testing.T) {
		ok, msg := ParseItemQuotaImpossibleErr(nil)
		assert.False(t, ok)
		assert.Empty(t, msg)
	})
}
