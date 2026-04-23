// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestChatProcessor_Transform(t *testing.T) {
	t.Parallel()
	p := &ChatProcessor{}

	t.Run("nil span - skip", func(t *testing.T) {
		spans := loop_span.SpanList{nil}
		result, err := p.Transform(context.Background(), spans)
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Nil(t, result[0])
	})

	t.Run("non-model span - input unchanged", func(t *testing.T) {
		originalInput := `{"messages":[{"role":"user","content":"Hello"}]}`
		spans := loop_span.SpanList{
			{SpanType: loop_span.SpanTypeTool, Input: originalInput},
			{SpanType: loop_span.SpanTypeFunction, Input: originalInput},
		}
		result, err := p.Transform(context.Background(), spans)
		require.NoError(t, err)
		assert.Equal(t, originalInput, result[0].Input)
		assert.Equal(t, originalInput, result[1].Input)
	})

	t.Run("model span - empty input returns empty", func(t *testing.T) {
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: ""}}
		result, err := p.Transform(context.Background(), spans)
		require.NoError(t, err)
		assert.Equal(t, "", result[0].Input)
	})

	t.Run("model span - invalid json returns no_query_parsed", func(t *testing.T) {
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: "not json"}}
		result, err := p.Transform(context.Background(), spans)
		require.NoError(t, err)
		assert.Equal(t, noQueryParsed, result[0].Input)
	})

	t.Run("model span - no messages or input field returns no_query_parsed", func(t *testing.T) {
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: `{"other":"field"}`}}
		result, err := p.Transform(context.Background(), spans)
		require.NoError(t, err)
		assert.Equal(t, noQueryParsed, result[0].Input)
	})
}

func TestChatProcessor_StandardChat(t *testing.T) {
	t.Parallel()
	p := &ChatProcessor{}
	ctx := context.Background()

	t.Run("last message is user - preserve structure with last user msg only", func(t *testing.T) {
		input := `{"messages":[{"role":"system","content":"You are helpful"},{"role":"user","content":"First"},{"role":"assistant","content":"Reply"},{"role":"user","content":"Second"}],"model":"gpt-4"}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, sonic.UnmarshalString(result[0].Input, &parsed))
		assert.Equal(t, "gpt-4", parsed["model"])
		messages, ok := parsed["messages"].([]interface{})
		require.True(t, ok)
		assert.Len(t, messages, 1)
		msg := messages[0].(map[string]interface{})
		assert.Equal(t, "user", msg["role"])
		assert.Equal(t, "Second", msg["content"])
	})

	t.Run("single user message - preserved", func(t *testing.T) {
		input := `{"messages":[{"role":"user","content":"Hello"}]}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, sonic.UnmarshalString(result[0].Input, &parsed))
		messages := parsed["messages"].([]interface{})
		assert.Len(t, messages, 1)
		msg := messages[0].(map[string]interface{})
		assert.Equal(t, "user", msg["role"])
		assert.Equal(t, "Hello", msg["content"])
	})

	t.Run("last message is assistant - input becomes empty", func(t *testing.T) {
		input := `{"messages":[{"role":"user","content":"Question"},{"role":"assistant","content":"Answer"}]}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)
		assert.Equal(t, "", result[0].Input)
	})

	t.Run("last message is system - input becomes empty", func(t *testing.T) {
		input := `{"messages":[{"role":"system","content":"You are helpful"}]}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)
		assert.Equal(t, "", result[0].Input)
	})

	t.Run("empty messages array - fallback no_query_parsed", func(t *testing.T) {
		input := `{"messages":[]}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)
		assert.Equal(t, noQueryParsed, result[0].Input)
	})
}

func TestChatProcessor_ResponsesAPI(t *testing.T) {
	t.Parallel()
	p := &ChatProcessor{}
	ctx := context.Background()

	t.Run("string input - return original input as-is", func(t *testing.T) {
		input := `{"input":"Hello from responses API","model":"gpt-4o"}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)
		assert.Equal(t, input, result[0].Input)
	})

	t.Run("empty string input - return original input as-is", func(t *testing.T) {
		input := `{"input":""}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)
		assert.Equal(t, input, result[0].Input)
	})

	t.Run("array input - last is user - preserve structure", func(t *testing.T) {
		input := `{"input":[{"type":"message","role":"assistant","content":"Prev"},{"type":"message","role":"user","content":"Query"}],"model":"gpt-4o"}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, sonic.UnmarshalString(result[0].Input, &parsed))
		assert.Equal(t, "gpt-4o", parsed["model"])
		items := parsed["input"].([]interface{})
		assert.Len(t, items, 1)
		item := items[0].(map[string]interface{})
		assert.Equal(t, "user", item["role"])
		assert.Equal(t, "Query", item["content"])
	})

	t.Run("array input - last is assistant - becomes empty", func(t *testing.T) {
		input := `{"input":[{"type":"message","role":"user","content":"Q"},{"type":"message","role":"assistant","content":"A"}]}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)
		assert.Equal(t, "", result[0].Input)
	})

	t.Run("array input - empty array - becomes empty", func(t *testing.T) {
		input := `{"input":[]}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)
		assert.Equal(t, "", result[0].Input)
	})

	t.Run("array input - last item not a map - becomes empty", func(t *testing.T) {
		input := `{"input":["just a string"]}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)
		assert.Equal(t, "", result[0].Input)
	})

	t.Run("input is number - not matched - no_query_parsed", func(t *testing.T) {
		input := `{"input":123}`
		spans := loop_span.SpanList{{SpanType: loop_span.SpanTypeModel, Input: input}}
		result, err := p.Transform(ctx, spans)
		require.NoError(t, err)
		assert.Equal(t, noQueryParsed, result[0].Input)
	})
}

func TestChatProcessor_MultipleSpans(t *testing.T) {
	t.Parallel()
	p := &ChatProcessor{}

	spans := loop_span.SpanList{
		{SpanType: loop_span.SpanTypeModel, Input: `{"messages":[{"role":"user","content":"Hello"}]}`},
		{SpanType: loop_span.SpanTypeTool, Input: `{"tool":"call"}`},
		nil,
		{SpanType: loop_span.SpanTypeModel, Input: ""},
	}

	result, err := p.Transform(context.Background(), spans)
	require.NoError(t, err)
	assert.Len(t, result, 4)

	assert.Contains(t, result[0].Input, "Hello")
	assert.Equal(t, `{"tool":"call"}`, result[1].Input)
	assert.Nil(t, result[2])
	assert.Equal(t, "", result[3].Input)
}

func TestNewChatProcessorFactory(t *testing.T) {
	factory := NewChatProcessorFactory()
	assert.NotNil(t, factory)

	processor, err := factory.CreateProcessor(context.Background(), Settings{})
	assert.NoError(t, err)
	assert.NotNil(t, processor)
	_, ok := processor.(*ChatProcessor)
	assert.True(t, ok)
}
