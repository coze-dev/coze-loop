// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package eino

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

const testResponseFormatJSONSchema = `{"name":"answer","schema":{"type":"object","properties":{"a":{"type":"string"}}},"strict":true}`

func TestQwenBuilder_ResponseFormatJSONSchema(t *testing.T) {
	model := &entity.Model{ProtocolConfig: &entity.ProtocolConfig{
		BaseURL: "https://example.invalid",
		APIKey:  "k",
		Model:   "qwen-test",
		ProtocolConfigQwen: &entity.ProtocolConfigQwen{
			ResponseFormatType:       ptr.Of("json_schema"),
			ResponseFormatJsonSchema: ptr.Of(testResponseFormatJSONSchema),
		},
	}}

	m, err := qwenBuilder(context.Background(), model)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}

func TestQianfanBuilder_ResponseFormatJSONSchema(t *testing.T) {
	model := &entity.Model{ProtocolConfig: &entity.ProtocolConfig{
		BaseURL: "https://example.invalid",
		APIKey:  "k",
		Model:   "qianfan-test",
		ProtocolConfigQianfan: &entity.ProtocolConfigQianfan{
			ResponseFormatType:       ptr.Of("json_schema"),
			ResponseFormatJsonSchema: ptr.Of(testResponseFormatJSONSchema),
		},
	}}

	m, err := qianfanBuilder(context.Background(), model)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}
