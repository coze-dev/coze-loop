// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
)

func TestEvalTargetUsage_Getters(t *testing.T) {
	t.Parallel()

	var nilUsage *EvalTargetUsage
	assert.Equal(t, int64(0), nilUsage.GetInputTokens())
	assert.Equal(t, int64(0), nilUsage.GetOutputTokens())
	assert.Equal(t, int64(0), nilUsage.GetTotalTokens())

	u := &EvalTargetUsage{InputTokens: 3, OutputTokens: 5, TotalTokens: 8}
	assert.Equal(t, int64(3), u.GetInputTokens())
	assert.Equal(t, int64(5), u.GetOutputTokens())
	assert.Equal(t, int64(8), u.GetTotalTokens())
}

func TestEvalTargetInputData_ValidateInputSchema_ExtraCases(t *testing.T) {
	t.Parallel()

	textType := ContentTypeText
	schema := []*ArgsSchema{
		{
			Key:                 gptr.Of("q"),
			SupportContentTypes: []ContentType{ContentTypeText},
			JsonSchema:          gptr.Of(`{"type":"string"}`),
		},
	}

	t.Run("nil content skipped", func(t *testing.T) {
		t.Parallel()
		in := &EvalTargetInputData{InputFields: map[string]*Content{"q": nil}}
		assert.NoError(t, in.ValidateInputSchema(schema))
	})

	t.Run("field absent from schema is skipped", func(t *testing.T) {
		t.Parallel()
		in := &EvalTargetInputData{InputFields: map[string]*Content{
			"other": {ContentType: &textType, Text: gptr.Of(`"hi"`)},
		}}
		assert.NoError(t, in.ValidateInputSchema(schema))
	})

	t.Run("nil content_type fails", func(t *testing.T) {
		t.Parallel()
		in := &EvalTargetInputData{InputFields: map[string]*Content{
			"q": {Text: gptr.Of(`"x"`)},
		}}
		err := in.ValidateInputSchema(schema)
		assert.Error(t, err)
	})

	t.Run("unsupported content type fails", func(t *testing.T) {
		t.Parallel()
		img := ContentTypeImage
		in := &EvalTargetInputData{InputFields: map[string]*Content{
			"q": {ContentType: &img},
		}}
		err := in.ValidateInputSchema(schema)
		assert.Error(t, err)
	})

	t.Run("invalid text against schema fails", func(t *testing.T) {
		t.Parallel()
		// Schema requires object, provide array literal.
		objSchema := []*ArgsSchema{
			{
				Key:                 gptr.Of("q"),
				SupportContentTypes: []ContentType{ContentTypeText},
				JsonSchema:          gptr.Of(`{"type":"object"}`),
			},
		}
		in := &EvalTargetInputData{InputFields: map[string]*Content{
			"q": {ContentType: &textType, Text: gptr.Of("[1,2,3]")},
		}}
		err := in.ValidateInputSchema(objSchema)
		assert.Error(t, err)
	})
}

func TestTargetTrajectoryConf_GetExtractInterval(t *testing.T) {
	t.Parallel()

	const defaultInterval = 15 * time.Second

	var nilConf *TargetTrajectoryConf
	assert.Equal(t, defaultInterval, nilConf.GetExtractInterval(1))

	empty := &TargetTrajectoryConf{}
	assert.Equal(t, defaultInterval, empty.GetExtractInterval(1))

	global := &TargetTrajectoryConf{ExtractIntervalSecond: 3}
	assert.Equal(t, 3*time.Second, global.GetExtractInterval(1))
	// space=0 falls through to global default
	assert.Equal(t, 3*time.Second, global.GetExtractInterval(0))

	perSpace := &TargetTrajectoryConf{
		ExtractIntervalSecond:      3,
		SpaceExtractIntervalSecond: map[int64]int64{99: 7},
	}
	assert.Equal(t, 7*time.Second, perSpace.GetExtractInterval(99))
	// space with 0 value falls back to global
	perSpace.SpaceExtractIntervalSecond[100] = 0
	assert.Equal(t, 3*time.Second, perSpace.GetExtractInterval(100))
}
