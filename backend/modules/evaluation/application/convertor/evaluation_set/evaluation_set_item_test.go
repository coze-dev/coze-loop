// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation_set

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	data_tag "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	eval_set_svc "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestItemDTO2DO_NilAndValues(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ItemDTO2DO(nil))
	assert.Nil(t, ItemDTO2DOs(nil))

	dto := &eval_set.EvaluationSetItem{
		ID:              gptr.Of(int64(1)),
		AppID:           gptr.Of(int32(2)),
		WorkspaceID:     gptr.Of(int64(3)),
		EvaluationSetID: gptr.Of(int64(4)),
		SchemaID:        gptr.Of(int64(5)),
		ItemID:          gptr.Of(int64(6)),
		ItemKey:         gptr.Of("k"),
		Turns: []*eval_set.Turn{
			{ID: gptr.Of(int64(11))},
		},
	}
	got := ItemDTO2DO(dto)
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(1), got.ID)
		assert.Equal(t, int32(2), got.AppID)
		assert.Equal(t, int64(3), got.SpaceID)
		assert.Equal(t, int64(4), got.EvaluationSetID)
		assert.Equal(t, "k", got.ItemKey)
		if assert.Len(t, got.Turns, 1) {
			// TurnDTO2DO fills evalSetID / itemID from parent
			assert.Equal(t, int64(4), got.Turns[0].EvalSetID)
			assert.Equal(t, int64(6), got.Turns[0].ItemID)
		}
	}

	list := ItemDTO2DOs([]*eval_set.EvaluationSetItem{nil, dto})
	// nil slice entries produce nil DOs (not filtered) — matches convention
	if assert.Len(t, list, 2) {
		assert.Nil(t, list[0])
	}
}

func TestItemDO2DTO_Roundtrip(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ItemDO2DTO(nil))
	assert.Nil(t, ItemDO2DTOs(nil))

	do := &entity.EvaluationSetItem{
		ID: 1, AppID: 2, SpaceID: 3, EvaluationSetID: 4, SchemaID: 5,
		ItemID: 6, ItemKey: "k",
		Turns: []*entity.Turn{{ID: 11}},
		Tags:  []*entity.ResourceTag{{TagName: "t"}},
	}
	dto := ItemDO2DTO(do)
	if assert.NotNil(t, dto) {
		assert.Equal(t, "k", *dto.ItemKey)
		assert.Len(t, dto.Turns, 1)
		assert.Len(t, dto.Tags, 1)
	}
}

func TestTurnDTO2DO_And_DO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, TurnDTO2DO(1, 2, nil))
	assert.Nil(t, TurnDO2DTO(nil))

	dto := &eval_set.Turn{ID: gptr.Of(int64(9))}
	got := TurnDTO2DO(4, 6, dto)
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(9), got.ID)
		assert.Equal(t, int64(4), got.EvalSetID)
		assert.Equal(t, int64(6), got.ItemID)
	}
}

func TestFieldData_Roundtrip(t *testing.T) {
	t.Parallel()

	assert.Nil(t, FieldDataDTO2DO(nil))
	assert.Nil(t, FieldDataDO2DTO(nil))
	assert.Nil(t, FieldDataDTO2DOs(nil))
	assert.Nil(t, FieldDataDO2DTOs(nil))

	do := FieldDataDTO2DO(&eval_set.FieldData{
		Key:     gptr.Of("k"),
		Name:    gptr.Of("n"),
		TraceID: gptr.Of("tid"),
	})
	if assert.NotNil(t, do) {
		assert.Equal(t, "k", do.Key)
		assert.Equal(t, "n", do.Name)
		assert.Equal(t, "tid", do.TraceID)
	}

	dto := FieldDataDO2DTO(&entity.FieldData{Key: "k", Name: "n", TraceID: "tid"})
	if assert.NotNil(t, dto) {
		assert.Equal(t, "k", *dto.Key)
		assert.Equal(t, "n", *dto.Name)
		assert.Equal(t, "tid", *dto.TraceID)
	}
}

func TestResourceTagDTO2DOs_Internal(t *testing.T) {
	t.Parallel()

	// nil / empty
	assert.Nil(t, resourceTagDTO2DOs(nil))
	assert.Empty(t, resourceTagDTO2DOs([]*eval_set.ResourceTag{}))

	// nil entries dropped; ContentType/Status carried through when non-nil
	ct := data_tag.TagContentType("text")
	st := data_tag.TagStatus("active")
	got := resourceTagDTO2DOs([]*eval_set.ResourceTag{
		nil,
		{TagName: "n", TagKeyID: gptr.Of(int64(7)), ContentType: gptr.Of(ct), Status: gptr.Of(st)},
	})
	if assert.Len(t, got, 1) {
		assert.Equal(t, "n", got[0].TagName)
		assert.Equal(t, int64(7), got[0].TagKeyID)
		assert.Equal(t, "text", got[0].ContentType)
		assert.Equal(t, "active", got[0].Status)
	}
}

func TestItemErrorGroupDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ItemErrorGroupDO2DTO(nil))
	assert.Nil(t, ItemErrorGroupDO2DTOs(nil))

	do := &entity.ItemErrorGroup{
		Type:       gptr.Of(entity.ItemErrorType_MismatchSchema),
		Summary:    gptr.Of("summary"),
		ErrorCount: gptr.Of(int32(2)),
		Details: []*entity.ItemErrorDetail{
			{Message: gptr.Of("m"), Index: gptr.Of(int32(1))},
		},
	}
	dto := ItemErrorGroupDO2DTO(do)
	if assert.NotNil(t, dto) {
		if assert.NotNil(t, dto.Type) {
			assert.Equal(t, dataset.ItemErrorType(entity.ItemErrorType_MismatchSchema), *dto.Type)
		}
		assert.Equal(t, "summary", *dto.Summary)
		assert.Equal(t, int32(2), *dto.ErrorCount)
		if assert.Len(t, dto.Details, 1) {
			assert.Equal(t, "m", *dto.Details[0].Message)
		}
	}

	// slice variant
	list := ItemErrorGroupDO2DTOs([]*entity.ItemErrorGroup{do})
	assert.Len(t, list, 1)
}

func TestItemErrorDetail(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ItemErrorDetailDO2DTO(nil))
	assert.Nil(t, ItemErrorDetailDO2DTOs(nil))

	do := &entity.ItemErrorDetail{
		Message:    gptr.Of("msg"),
		Index:      gptr.Of(int32(1)),
		StartIndex: gptr.Of(int32(2)),
		EndIndex:   gptr.Of(int32(3)),
	}
	dto := ItemErrorDetailDO2DTO(do)
	if assert.NotNil(t, dto) {
		assert.Equal(t, "msg", *dto.Message)
		assert.Equal(t, int32(2), *dto.StartIndex)
		assert.Equal(t, int32(3), *dto.EndIndex)
	}
}

func TestItemDefDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ItemDefDO2DTO(nil))
	assert.Nil(t, ItemDefDO2DTOs(nil))

	do := &entity.EvaluationSetItemDef{
		ItemID: 1, SpaceID: 2, EvaluationSetID: 3,
		ItemKey: "k", Status: "s", LatestVersion: "v",
	}
	dto := ItemDefDO2DTO(do)
	if assert.NotNil(t, dto) {
		assert.Equal(t, int64(1), *dto.ItemID)
		assert.Equal(t, "k", *dto.ItemKey)
		assert.Equal(t, "s", *dto.Status)
		assert.Equal(t, "v", *dto.LatestVersion)
	}

	list := ItemDefDO2DTOs([]*entity.EvaluationSetItemDef{do})
	assert.Len(t, list, 1)
}

func TestItemVersionDO2DTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ItemVersionDO2DTO(nil))
	assert.Nil(t, ItemVersionDO2DTOs(nil))

	do := &entity.EvaluationSetItemVersion{
		ItemVersionID: 11, ItemID: 22, Version: "v", VersionNum: 1,
		Description: "d", Status: "s",
		Turns: []*entity.Turn{{ID: 1}},
	}
	dto := ItemVersionDO2DTO(do)
	if assert.NotNil(t, dto) {
		assert.Equal(t, int64(11), *dto.ItemVersionID)
		assert.Equal(t, int64(22), *dto.ItemID)
		assert.Equal(t, "v", *dto.Version)
		assert.Len(t, dto.Turns, 1)
	}
	assert.Len(t, ItemVersionDO2DTOs([]*entity.EvaluationSetItemVersion{do}), 1)
}

func TestItemVersionRef_Roundtrip(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ItemVersionRefDTO2DO(nil))
	assert.Nil(t, ItemVersionRefDO2DTO(nil))
	assert.Nil(t, ItemVersionRefDTO2DOs(nil))
	assert.Nil(t, ItemVersionRefDO2DTOs(nil))

	dto := &eval_set_svc.EvaluationItemVersionRef{
		ItemID:        123,
		ItemVersionID: gptr.Of(int64(456)),
		ItemVersion:   gptr.Of("v1"),
	}
	do := ItemVersionRefDTO2DO(dto)
	if assert.NotNil(t, do) {
		assert.Equal(t, int64(123), do.ItemID)
		if assert.NotNil(t, do.ItemVersionID) {
			assert.Equal(t, int64(456), *do.ItemVersionID)
		}
		if assert.NotNil(t, do.ItemVersion) {
			assert.Equal(t, "v1", *do.ItemVersion)
		}
	}

	// DO → DTO round-trip
	backDTO := ItemVersionRefDO2DTO(do)
	if assert.NotNil(t, backDTO) {
		assert.Equal(t, dto.ItemID, backDTO.ItemID)
		assert.Equal(t, dto.ItemVersionID, backDTO.ItemVersionID)
		assert.Equal(t, dto.ItemVersion, backDTO.ItemVersion)
	}

	// slice helpers
	dtos := ItemVersionRefDO2DTOs([]*entity.EvaluationItemVersionRef{do})
	assert.Len(t, dtos, 1)
	dos := ItemVersionRefDTO2DOs([]*eval_set_svc.EvaluationItemVersionRef{dto})
	assert.Len(t, dos, 1)
}
