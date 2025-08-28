// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	code "github.com/coze-dev/coze-loop/backend/pkg/errorx/code"
)

// simpleBizErr implements github.com/cloudwego/kitex/pkg/kerrors.BizStatusErrorIface
type simpleBizErr struct {
	code  int32
	msg   string
	extra map[string]string
}

func (e *simpleBizErr) Error() string               { return e.msg }
func (e *simpleBizErr) BizStatusCode() int32        { return e.code }
func (e *simpleBizErr) BizMessage() string          { return e.msg }
func (e *simpleBizErr) BizExtra() map[string]string { return e.extra }

func TestGetCode_Nil(t *testing.T) {
	codeVal, isErr := GetCode(nil)
	assert.Equal(t, int64(0), codeVal)
	assert.Equal(t, int64(0), isErr)
}

func TestGetCode_WithBizStatusErrorIface_ExtraAffectStability(t *testing.T) {
	// isError should follow biz_err_affect_stability when provided
	err := &simpleBizErr{
		code:  12345,
		msg:   "biz error",
		extra: map[string]string{"biz_err_affect_stability": "1"},
	}
	codeVal, isErr := GetCode(err)
	assert.Equal(t, int64(12345), codeVal)
	assert.Equal(t, int64(1), isErr)

	// when explicitly set to 0
	err0 := &simpleBizErr{
		code:  23456,
		msg:   "biz error",
		extra: map[string]string{"biz_err_affect_stability": "0"},
	}
	codeVal, isErr = GetCode(err0)
	assert.Equal(t, int64(23456), codeVal)
	assert.Equal(t, int64(0), isErr)
}

func TestGetCode_WithBizStatusErrorIface_EmptyExtra(t *testing.T) {
	// If extra is empty, isError keeps previous default (1)
	err := &simpleBizErr{
		code:  34567,
		msg:   "biz error without extra",
		extra: map[string]string{},
	}
	codeVal, isErr := GetCode(err)
	assert.Equal(t, int64(34567), codeVal)
	assert.Equal(t, int64(1), isErr)
}

func TestGetCode_ErrorxNewByCode_AffectStabilityFalse(t *testing.T) {
	// Register a custom code whose stability does NOT affect error rate
	const c int32 = 770001
	code.Register(c, "test code", code.WithAffectStability(false))

	err := errorx.NewByCode(c)
	codeVal, isErr := GetCode(err)
	assert.Equal(t, int64(c), codeVal)
	assert.Equal(t, int64(0), isErr)
}

func TestGetCode_ErrorxNewByCode_AffectStabilityTrue(t *testing.T) {
	// Register a custom code whose stability DOES affect error rate
	const c int32 = 770002
	code.Register(c, "test code", code.WithAffectStability(true))

	err := errorx.NewByCode(c)
	codeVal, isErr := GetCode(err)
	assert.Equal(t, int64(c), codeVal)
	assert.Equal(t, int64(1), isErr)
}
