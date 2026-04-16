// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertToolDTO2DO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertToolDTO2DO(nil))
}

func TestConvertToolDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertToolDO2DTO(nil))
}

func TestConvertFunctionDTO2DO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertFunctionDTO2DO(nil))
}

func TestConvertFunctionDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertFunctionDO2DTO(nil))
}
