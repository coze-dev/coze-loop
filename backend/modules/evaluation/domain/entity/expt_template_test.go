// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
)

func TestExptTemplate_ToEvaluatorRefDO(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			Meta: &ExptTemplateMeta{
				ID:          1,
				WorkspaceID: 2,
			},
			EvaluatorVersionRef: []*ExptTemplateEvaluatorVersionRef{
				{EvaluatorID: 3, EvaluatorVersionID: 4},
				{EvaluatorID: 5, EvaluatorVersionID: 6},
			},
		}
		refs := e.ToEvaluatorRefDO()
		assert.Len(t, refs, 2)
		assert.Equal(t, int64(2), refs[0].SpaceID)
		assert.Equal(t, int64(1), refs[0].ExptTemplateID)
		assert.Equal(t, int64(3), refs[0].EvaluatorID)
		assert.Equal(t, int64(4), refs[0].EvaluatorVersionID)
		assert.Equal(t, int64(5), refs[1].EvaluatorID)
		assert.Equal(t, int64(6), refs[1].EvaluatorVersionID)
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Nil(t, e.ToEvaluatorRefDO())
	})

	t.Run("empty refs", func(t *testing.T) {
		e := &ExptTemplate{
			Meta:                &ExptTemplateMeta{ID: 1, WorkspaceID: 2},
			EvaluatorVersionRef: []*ExptTemplateEvaluatorVersionRef{},
		}
		refs := e.ToEvaluatorRefDO()
		assert.NotNil(t, refs)
		assert.Len(t, refs, 0)
	})
}

func TestExptTemplate_ContainsEvalTarget(t *testing.T) {
	t.Run("contains target", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{
				TargetVersionID: 100,
			},
		}
		assert.True(t, e.ContainsEvalTarget())
	})

	t.Run("no target", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{
				TargetVersionID: 0,
			},
		}
		assert.False(t, e.ContainsEvalTarget())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.False(t, e.ContainsEvalTarget())
	})

	t.Run("nil triple config", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: nil,
		}
		assert.False(t, e.ContainsEvalTarget())
	})
}

func TestExptTemplate_GetID(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			Meta: &ExptTemplateMeta{ID: 123},
		}
		assert.Equal(t, int64(123), e.GetID())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, int64(0), e.GetID())
	})

	t.Run("nil meta", func(t *testing.T) {
		e := &ExptTemplate{Meta: nil}
		assert.Equal(t, int64(0), e.GetID())
	})
}

func TestExptTemplate_GetSpaceID(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			Meta: &ExptTemplateMeta{WorkspaceID: 456},
		}
		assert.Equal(t, int64(456), e.GetSpaceID())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, int64(0), e.GetSpaceID())
	})

	t.Run("nil meta", func(t *testing.T) {
		e := &ExptTemplate{Meta: nil}
		assert.Equal(t, int64(0), e.GetSpaceID())
	})
}

func TestExptTemplate_GetName(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			Meta: &ExptTemplateMeta{Name: "test_template"},
		}
		assert.Equal(t, "test_template", e.GetName())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, "", e.GetName())
	})

	t.Run("nil meta", func(t *testing.T) {
		e := &ExptTemplate{Meta: nil}
		assert.Equal(t, "", e.GetName())
	})
}

func TestExptTemplate_GetDescription(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			Meta: &ExptTemplateMeta{Desc: "test description"},
		}
		assert.Equal(t, "test description", e.GetDescription())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, "", e.GetDescription())
	})

	t.Run("nil meta", func(t *testing.T) {
		e := &ExptTemplate{Meta: nil}
		assert.Equal(t, "", e.GetDescription())
	})
}

func TestExptTemplate_GetCreatedBy(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			BaseInfo: &BaseInfo{
				CreatedBy: &UserInfo{UserID: gptr.Of("user123")},
			},
		}
		assert.Equal(t, "user123", e.GetCreatedBy())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, "", e.GetCreatedBy())
	})

	t.Run("nil base info", func(t *testing.T) {
		e := &ExptTemplate{BaseInfo: nil}
		assert.Equal(t, "", e.GetCreatedBy())
	})

	t.Run("nil created by", func(t *testing.T) {
		e := &ExptTemplate{
			BaseInfo: &BaseInfo{CreatedBy: nil},
		}
		assert.Equal(t, "", e.GetCreatedBy())
	})

	t.Run("nil user id", func(t *testing.T) {
		e := &ExptTemplate{
			BaseInfo: &BaseInfo{
				CreatedBy: &UserInfo{UserID: nil},
			},
		}
		assert.Equal(t, "", e.GetCreatedBy())
	})
}

func TestExptTemplate_GetExptType(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			Meta: &ExptTemplateMeta{ExptType: ExptType_Offline},
		}
		assert.Equal(t, ExptType_Offline, e.GetExptType())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, ExptType(0), e.GetExptType())
	})

	t.Run("nil meta", func(t *testing.T) {
		e := &ExptTemplate{Meta: nil}
		assert.Equal(t, ExptType(0), e.GetExptType())
	})
}

func TestExptTemplate_GetEvalSetID(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{EvalSetID: 789},
		}
		assert.Equal(t, int64(789), e.GetEvalSetID())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, int64(0), e.GetEvalSetID())
	})

	t.Run("nil triple config", func(t *testing.T) {
		e := &ExptTemplate{TripleConfig: nil}
		assert.Equal(t, int64(0), e.GetEvalSetID())
	})
}

func TestExptTemplate_GetEvalSetVersionID(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{EvalSetVersionID: 101},
		}
		assert.Equal(t, int64(101), e.GetEvalSetVersionID())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, int64(0), e.GetEvalSetVersionID())
	})

	t.Run("nil triple config", func(t *testing.T) {
		e := &ExptTemplate{TripleConfig: nil}
		assert.Equal(t, int64(0), e.GetEvalSetVersionID())
	})
}

func TestExptTemplate_GetTargetID(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{TargetID: 202},
		}
		assert.Equal(t, int64(202), e.GetTargetID())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, int64(0), e.GetTargetID())
	})

	t.Run("nil triple config", func(t *testing.T) {
		e := &ExptTemplate{TripleConfig: nil}
		assert.Equal(t, int64(0), e.GetTargetID())
	})
}

func TestExptTemplate_GetTargetVersionID(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{TargetVersionID: 303},
		}
		assert.Equal(t, int64(303), e.GetTargetVersionID())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, int64(0), e.GetTargetVersionID())
	})

	t.Run("nil triple config", func(t *testing.T) {
		e := &ExptTemplate{TripleConfig: nil}
		assert.Equal(t, int64(0), e.GetTargetVersionID())
	})
}

func TestExptTemplate_GetTargetType(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{TargetType: EvalTargetTypeCozeBot},
		}
		assert.Equal(t, EvalTargetTypeCozeBot, e.GetTargetType())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Equal(t, EvalTargetType(0), e.GetTargetType())
	})

	t.Run("nil triple config", func(t *testing.T) {
		e := &ExptTemplate{TripleConfig: nil}
		assert.Equal(t, EvalTargetType(0), e.GetTargetType())
	})
}

func TestExptTemplate_GetEvaluatorVersionIds(t *testing.T) {
	t.Run("from EvaluatorIDVersionItems", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{
				EvaluatorIDVersionItems: []*EvaluatorIDVersionItem{
					{EvaluatorVersionID: 1},
					{EvaluatorVersionID: 2},
					nil,                     // nil item should be skipped
					{EvaluatorVersionID: 0}, // zero version id should be skipped
					{EvaluatorVersionID: 3},
				},
				EvaluatorVersionIds: []int64{99, 100}, // should be ignored
			},
		}
		ids := e.GetEvaluatorVersionIds()
		assert.Equal(t, []int64{1, 2, 3}, ids)
	})

	t.Run("fallback to EvaluatorVersionIds", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{
				EvaluatorIDVersionItems: nil,
				EvaluatorVersionIds:     []int64{4, 5, 6},
			},
		}
		ids := e.GetEvaluatorVersionIds()
		assert.Equal(t, []int64{4, 5, 6}, ids)
	})

	t.Run("empty EvaluatorIDVersionItems", func(t *testing.T) {
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{
				EvaluatorIDVersionItems: []*EvaluatorIDVersionItem{},
				EvaluatorVersionIds:     []int64{7, 8},
			},
		}
		ids := e.GetEvaluatorVersionIds()
		assert.Equal(t, []int64{7, 8}, ids)
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Nil(t, e.GetEvaluatorVersionIds())
	})

	t.Run("nil triple config", func(t *testing.T) {
		e := &ExptTemplate{TripleConfig: nil}
		assert.Nil(t, e.GetEvaluatorVersionIds())
	})
}

func TestExptTemplate_GetEvaluatorIDVersionItems(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		items := []*EvaluatorIDVersionItem{
			{EvaluatorID: 1, Version: "v1", EvaluatorVersionID: 10},
			{EvaluatorID: 2, Version: "v2", EvaluatorVersionID: 20},
		}
		e := &ExptTemplate{
			TripleConfig: &ExptTemplateTuple{
				EvaluatorIDVersionItems: items,
			},
		}
		assert.Equal(t, items, e.GetEvaluatorIDVersionItems())
	})

	t.Run("nil template", func(t *testing.T) {
		var e *ExptTemplate
		assert.Nil(t, e.GetEvaluatorIDVersionItems())
	})

	t.Run("nil triple config", func(t *testing.T) {
		e := &ExptTemplate{TripleConfig: nil}
		assert.Nil(t, e.GetEvaluatorIDVersionItems())
	})
}

func TestExptTemplateEvaluatorVersionRef_String(t *testing.T) {
	ref := &ExptTemplateEvaluatorVersionRef{
		EvaluatorID:        123,
		EvaluatorVersionID: 456,
	}
	str := ref.String()
	assert.Contains(t, str, "evaluator_id=")
	assert.Contains(t, str, "evaluator_version_id=")
	assert.Contains(t, str, "123")
	assert.Contains(t, str, "456")
}

func TestExptTemplateUpdateFields_ToFieldMap(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		fields := &ExptTemplateUpdateFields{
			Name:        "test_name",
			Description: "test_desc",
		}
		m, err := fields.ToFieldMap()
		assert.NoError(t, err)
		assert.NotNil(t, m)
		assert.Equal(t, "test_name", m["name"])
		assert.Equal(t, "test_desc", m["description"])
	})

	t.Run("empty fields", func(t *testing.T) {
		fields := &ExptTemplateUpdateFields{}
		m, err := fields.ToFieldMap()
		assert.NoError(t, err)
		assert.NotNil(t, m)
	})
}

func TestExptTemplateConfiguration_Valid(t *testing.T) {
	ctx := context.Background()

	t.Run("nil config", func(t *testing.T) {
		var c *ExptTemplateConfiguration
		err := c.Valid(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil ExptTemplateConfiguration")
	})

	t.Run("valid config", func(t *testing.T) {
		itemConcurNum := 5
		evaluatorsConcurNum := 3
		c := &ExptTemplateConfiguration{
			ItemConcurNum:       &itemConcurNum,
			EvaluatorsConcurNum: &evaluatorsConcurNum,
		}
		err := c.Valid(ctx)
		assert.NoError(t, err)
	})

	t.Run("invalid item_concur_num", func(t *testing.T) {
		zero := 0
		negative := -1
		testCases := []struct {
			name string
			num  *int
		}{
			{"zero", &zero},
			{"negative", &negative},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := &ExptTemplateConfiguration{
					ItemConcurNum: tc.num,
				}
				err := c.Valid(ctx)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "item_concur_num must be greater than 0")
			})
		}
	})

	t.Run("invalid evaluators_concur_num", func(t *testing.T) {
		zero := 0
		negative := -1
		testCases := []struct {
			name string
			num  *int
		}{
			{"zero", &zero},
			{"negative", &negative},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := &ExptTemplateConfiguration{
					EvaluatorsConcurNum: tc.num,
				}
				err := c.Valid(ctx)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "evaluators_concur_num must be greater than 0")
			})
		}
	})

	t.Run("valid with EvaluatorsConf", func(t *testing.T) {
		itemConcurNum := 2
		c := &ExptTemplateConfiguration{
			ItemConcurNum: &itemConcurNum,
			ConnectorConf: Connector{
				EvaluatorsConf: &EvaluatorsConf{
					EvaluatorConf: []*EvaluatorConf{
						{
							EvaluatorVersionID: 1,
							IngressConf: &EvaluatorIngressConf{
								TargetAdapter:  &FieldAdapter{},
								EvalSetAdapter: &FieldAdapter{},
							},
						},
					},
				},
			},
		}
		err := c.Valid(ctx)
		assert.NoError(t, err)
	})

	t.Run("invalid EvaluatorsConf", func(t *testing.T) {
		itemConcurNum := 2
		c := &ExptTemplateConfiguration{
			ItemConcurNum: &itemConcurNum,
			ConnectorConf: Connector{
				EvaluatorsConf: &EvaluatorsConf{
					EvaluatorConf: []*EvaluatorConf{
						{
							EvaluatorVersionID: 1,
							IngressConf:        nil, // invalid
						},
					},
				},
			},
		}
		err := c.Valid(ctx)
		assert.Error(t, err)
	})
}

func TestExptTemplateConfiguration_GetDefaultItemConcurNum(t *testing.T) {
	t.Run("with valid value", func(t *testing.T) {
		num := 5
		c := &ExptTemplateConfiguration{
			ItemConcurNum: &num,
		}
		assert.Equal(t, 5, c.GetDefaultItemConcurNum())
	})

	t.Run("nil config", func(t *testing.T) {
		var c *ExptTemplateConfiguration
		assert.Equal(t, 1, c.GetDefaultItemConcurNum())
	})

	t.Run("nil ItemConcurNum", func(t *testing.T) {
		c := &ExptTemplateConfiguration{
			ItemConcurNum: nil,
		}
		assert.Equal(t, 1, c.GetDefaultItemConcurNum())
	})

	t.Run("zero ItemConcurNum", func(t *testing.T) {
		zero := 0
		c := &ExptTemplateConfiguration{
			ItemConcurNum: &zero,
		}
		assert.Equal(t, 1, c.GetDefaultItemConcurNum())
	})

	t.Run("negative ItemConcurNum", func(t *testing.T) {
		negative := -1
		c := &ExptTemplateConfiguration{
			ItemConcurNum: &negative,
		}
		assert.Equal(t, 1, c.GetDefaultItemConcurNum())
	})
}

func TestExptTemplateConfiguration_GetDefaultEvaluatorsConcurNum(t *testing.T) {
	t.Run("with valid value", func(t *testing.T) {
		num := 7
		c := &ExptTemplateConfiguration{
			EvaluatorsConcurNum: &num,
		}
		assert.Equal(t, 7, c.GetDefaultEvaluatorsConcurNum())
	})

	t.Run("nil config", func(t *testing.T) {
		var c *ExptTemplateConfiguration
		assert.Equal(t, 3, c.GetDefaultEvaluatorsConcurNum())
	})

	t.Run("nil EvaluatorsConcurNum", func(t *testing.T) {
		c := &ExptTemplateConfiguration{
			EvaluatorsConcurNum: nil,
		}
		assert.Equal(t, 3, c.GetDefaultEvaluatorsConcurNum())
	})

	t.Run("zero EvaluatorsConcurNum", func(t *testing.T) {
		zero := 0
		c := &ExptTemplateConfiguration{
			EvaluatorsConcurNum: &zero,
		}
		assert.Equal(t, 3, c.GetDefaultEvaluatorsConcurNum())
	})

	t.Run("negative EvaluatorsConcurNum", func(t *testing.T) {
		negative := -1
		c := &ExptTemplateConfiguration{
			EvaluatorsConcurNum: &negative,
		}
		assert.Equal(t, 3, c.GetDefaultEvaluatorsConcurNum())
	})
}
