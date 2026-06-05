// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
)

func TestFilterFieldsDTO2DO_IsCustom(t *testing.T) {
	isCustomTrue := true
	fieldName := "test_field"
	f := &filter.FilterFields{
		FilterFields: []*filter.FilterField{
			{
				FieldName: &fieldName,
				IsCustom:  &isCustomTrue,
			},
		},
	}

	result := FilterFieldsDTO2DO(f)

	assert.NotNil(t, result)
	assert.Len(t, result.FilterFields, 1)
	assert.True(t, result.FilterFields[0].IsCustom)
}

func TestFilterFieldsDTO2DO_IsCustomFalse(t *testing.T) {
	isCustomFalse := false
	fieldName := "test_field"
	f := &filter.FilterFields{
		FilterFields: []*filter.FilterField{
			{
				FieldName: &fieldName,
				IsCustom:  &isCustomFalse,
			},
		},
	}

	result := FilterFieldsDTO2DO(f)

	assert.NotNil(t, result)
	assert.Len(t, result.FilterFields, 1)
	assert.False(t, result.FilterFields[0].IsCustom)
}

func TestFilterFieldsDTO2DO_IsCustomNil(t *testing.T) {
	fieldName := "test_field"
	f := &filter.FilterFields{
		FilterFields: []*filter.FilterField{
			{
				FieldName: &fieldName,
				Values:    []string{"val1"},
			},
		},
	}

	result := FilterFieldsDTO2DO(f)

	assert.NotNil(t, result)
	assert.Len(t, result.FilterFields, 1)
	assert.False(t, result.FilterFields[0].IsCustom) // default zero value
}
