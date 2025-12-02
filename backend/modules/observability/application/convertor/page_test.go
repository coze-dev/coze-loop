// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
)

func TestOrderByDTO2DO(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		result := OrderByDTO2DO(nil)
		assert.Nil(t, result)
	})

	t.Run("valid input", func(t *testing.T) {
		t.Parallel()
		orderBy := &common.OrderBy{
			Field: ptr.Of("created_at"),
			IsAsc: ptr.Of(true),
		}
		result := OrderByDTO2DO(orderBy)
		assert.NotNil(t, result)
		assert.Equal(t, "created_at", result.Field)
		assert.True(t, result.IsAsc)
	})

	t.Run("descending order", func(t *testing.T) {
		t.Parallel()
		orderBy := &common.OrderBy{
			Field: ptr.Of("updated_at"),
			IsAsc: ptr.Of(false),
		}
		result := OrderByDTO2DO(orderBy)
		assert.NotNil(t, result)
		assert.Equal(t, "updated_at", result.Field)
		assert.False(t, result.IsAsc)
	})
}

func TestOrderByDO2DTO(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		result := OrderByDO2DTO(nil)
		assert.Nil(t, result)
	})

	t.Run("valid input", func(t *testing.T) {
		t.Parallel()
		orderBy := &entity.OrderBy{
			Field: "name",
			IsAsc: true,
		}
		result := OrderByDO2DTO(orderBy)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Field)
		assert.Equal(t, "name", *result.Field)
		assert.NotNil(t, result.IsAsc)
		assert.True(t, *result.IsAsc)
	})

	t.Run("descending order", func(t *testing.T) {
		t.Parallel()
		orderBy := &entity.OrderBy{
			Field: "id",
			IsAsc: false,
		}
		result := OrderByDO2DTO(orderBy)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Field)
		assert.Equal(t, "id", *result.Field)
		assert.NotNil(t, result.IsAsc)
		assert.False(t, *result.IsAsc)
	})
}

func TestOrderByConversionRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("round trip conversion", func(t *testing.T) {
		t.Parallel()
		original := &common.OrderBy{
			Field: ptr.Of("test_field"),
			IsAsc: ptr.Of(true),
		}
		// DTO -> DO -> DTO
		do := OrderByDTO2DO(original)
		assert.NotNil(t, do)

		result := OrderByDO2DTO(do)
		assert.NotNil(t, result)
		assert.Equal(t, *original.Field, *result.Field)
		assert.Equal(t, *original.IsAsc, *result.IsAsc)
	})

	t.Run("round trip with entity", func(t *testing.T) {
		t.Parallel()
		original := &entity.OrderBy{
			Field: "entity_field",
			IsAsc: false,
		}

		// DO -> DTO -> DO
		dto := OrderByDO2DTO(original)
		assert.NotNil(t, dto)

		result := OrderByDTO2DO(dto)
		assert.NotNil(t, result)
		assert.Equal(t, original.Field, result.Field)
		assert.Equal(t, original.IsAsc, result.IsAsc)
	})
}
