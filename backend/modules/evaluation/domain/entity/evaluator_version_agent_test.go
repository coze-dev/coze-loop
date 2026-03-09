// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestAgentEvaluatorVersion_ValidateInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		version   *AgentEvaluatorVersion
		input     *EvaluatorInputData
		wantErr   bool
		wantCode  int32
		assertion func(t *testing.T, err error)
	}{
		{
			name: "ignore fields not in schema",
			version: &AgentEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{Key: gptr.Of("defined"), SupportContentTypes: []ContentType{ContentTypeText}, JsonSchema: gptr.Of(`{"type":"string"}`)},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"undefined": {ContentType: gptr.Of(ContentTypeImage)},
				},
			},
		},
		{
			name: "skip nil content",
			version: &AgentEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{Key: gptr.Of("defined"), SupportContentTypes: []ContentType{ContentTypeText}, JsonSchema: gptr.Of(`{"type":"string"}`)},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"defined": nil,
				},
			},
		},
		{
			name: "content type not supported",
			version: &AgentEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{Key: gptr.Of("f1"), SupportContentTypes: []ContentType{ContentTypeText}, JsonSchema: gptr.Of(`{"type":"string"}`)},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"f1": {ContentType: gptr.Of(ContentTypeImage)},
				},
			},
			wantErr:  true,
			wantCode: errno.ContentTypeNotSupportedCode,
		},
		{
			name: "content schema invalid",
			version: &AgentEvaluatorVersion{
				InputSchemas: []*ArgsSchema{
					{Key: gptr.Of("f1"), SupportContentTypes: []ContentType{ContentTypeText}, JsonSchema: gptr.Of(`{"type":"number"}`)},
				},
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"f1": {ContentType: gptr.Of(ContentTypeText), Text: gptr.Of("hello")},
				},
			},
			wantErr:  true,
			wantCode: errno.ContentSchemaInvalidCode,
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.version.ValidateInput(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantCode != 0 {
					se, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantCode, se.Code())
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestAgentEvaluatorVersion_ValidateBaseInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		version  *AgentEvaluatorVersion
		wantErr  bool
		wantCode int32
	}{
		{
			name:     "nil receiver",
			version:  nil,
			wantErr:  true,
			wantCode: errno.EvaluatorNotExistCode,
		},
		{
			name:    "non-nil ok",
			version: &AgentEvaluatorVersion{},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.version.ValidateBaseInfo()
			if tc.wantErr {
				assert.Error(t, err)
				se, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, tc.wantCode, se.Code())
				return
			}
			assert.NoError(t, err)
		})
	}
}

