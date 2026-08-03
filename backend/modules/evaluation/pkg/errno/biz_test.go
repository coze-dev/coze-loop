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
