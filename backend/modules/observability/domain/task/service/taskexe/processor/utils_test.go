// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/evaluationset"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	taskentity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
)

func TestGetCategory(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		typ      task.TaskType
		expected entity.DatasetCategory
	}{
		{"auto_eval", task.TaskTypeAutoEval, entity.DatasetCategory_Evaluation},
		{"other", "unknown", entity.DatasetCategory_General},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, getCategory(tt.typ))
		})
	}
}

func TestShouldTriggerBackfill(t *testing.T) {
	t.Parallel()

	baseTask := &taskentity.ObservabilityTask{
		TaskType: task.TaskTypeAutoEval,
		BackfillEffectiveTime: &taskentity.EffectiveTime{
			StartAt: time.Now().Add(-time.Hour).UnixMilli(),
			EndAt:   time.Now().Add(time.Hour).UnixMilli(),
		},
	}

	cases := []struct {
		name     string
		task     *taskentity.ObservabilityTask
		expected bool
	}{
		{"nil_time", &taskentity.ObservabilityTask{TaskType: taskentity.TaskTypeAutoEval}, false},
		{"invalid_type", &taskentity.ObservabilityTask{TaskType: taskentity.TaskType("manual")}, false},
		{"invalid_range", &taskentity.ObservabilityTask{TaskType: taskentity.TaskTypeAutoEval, BackfillEffectiveTime: &taskentity.EffectiveTime{StartAt: 10, EndAt: 5}}, false},
		{"valid", baseTask, true},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ShouldTriggerBackfill(tt.task))
		})
	}
}

func TestShouldTriggerNewData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	now := time.Now()
	baseTask := &taskentity.ObservabilityTask{
		TaskType: task.TaskTypeAutoEval,
		EffectiveTime: &taskentity.EffectiveTime{
			StartAt: now.Add(-time.Hour).UnixMilli(),
			EndAt:   now.Add(time.Hour).UnixMilli(),
		},
	}

	cases := []struct {
		name     string
		task     *taskentity.ObservabilityTask
		expected bool
	}{
		{"invalid_type", &taskentity.ObservabilityTask{TaskType: taskentity.TaskType("manual")}, false},
		{"nil_time", &taskentity.ObservabilityTask{TaskType: taskentity.TaskTypeAutoEval}, false},
		{"invalid_range", &taskentity.ObservabilityTask{TaskType: taskentity.TaskTypeAutoEval, EffectiveTime: &taskentity.EffectiveTime{StartAt: 20, EndAt: 10}}, false},
		{"start_in_future", &taskentity.ObservabilityTask{TaskType: taskentity.TaskTypeAutoEval, EffectiveTime: &taskentity.EffectiveTime{StartAt: now.Add(time.Hour).UnixMilli(), EndAt: now.Add(2 * time.Hour).UnixMilli()}}, false},
		{"valid", baseTask, true},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ShouldTriggerNewData(ctx, tt.task))
		})
	}
}

func TestToJSONString(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	assert.Equal(t, "", ToJSONString(ctx, nil))

	type sample struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	assert.Equal(t, "{\"name\":\"foo\",\"value\":123}", ToJSONString(ctx, sample{Name: "foo", Value: 123}))

	bad := map[string]interface{}{"ch": make(chan int)}
	assert.Equal(t, "", ToJSONString(ctx, bad))
}

func TestGetBasicEvaluationSetSchema(t *testing.T) {
	t.Parallel()

	columns := []string{"trace_id", "span_id"}
	schema, mappings := getBasicEvaluationSetSchema(columns)

	assert.Len(t, schema.FieldSchemas, 2)
	for i, column := range columns {
		fs := schema.FieldSchemas[i]
		assert.Equal(t, column, fs.GetKey())
		assert.Equal(t, column, fs.GetName())
		assert.Equal(t, column, fs.GetDescription())
		assert.Equal(t, common.ContentTypeText, fs.GetContentType())
		assert.Equal(t, "{\"type\": \"string\"}", fs.GetTextSchema())

		fm := mappings[i]
		assert.Equal(t, column, fm.GetFieldName())
		assert.Equal(t, column, fm.GetFromFieldName())
	}
}

func TestConvertDatasetSchemaDTO2DO(t *testing.T) {
	t.Parallel()

	assert.Empty(t, convertDatasetSchemaDTO2DO(nil).FieldSchemas)

	schema := dataset.NewDatasetSchema()
	schema.FieldSchemas = []*dataset.FieldSchema{
		{
			Name:        gptr.Of("field_a"),
			Description: gptr.Of("desc"),
			ContentType: gptr.Of(common.ContentTypeImage),
			TextSchema:  gptr.Of("{}"),
		},
		{
			Key:         gptr.Of("key_b"),
			Name:        gptr.Of("field_b"),
			Description: gptr.Of("desc_b"),
			ContentType: gptr.Of(common.ContentTypeAudio),
			TextSchema:  gptr.Of("{\"type\":\"number\"}"),
		},
	}

	result := convertDatasetSchemaDTO2DO(schema)
	assert.Len(t, result.FieldSchemas, 2)
	assert.Equal(t, "field_a", *result.FieldSchemas[0].Key)
	assert.Equal(t, "field_a", result.FieldSchemas[0].Name)
	assert.Equal(t, entity.ContentType_Image, result.FieldSchemas[0].ContentType)
	assert.Equal(t, "{}", result.FieldSchemas[0].TextSchema)

	assert.Equal(t, "key_b", *result.FieldSchemas[1].Key)
	assert.Equal(t, entity.ContentType_Audio, result.FieldSchemas[1].ContentType)
	assert.Equal(t, "{\"type\":\"number\"}", result.FieldSchemas[1].TextSchema)
}

func TestConvertContentTypeDTO2DO(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input    common.ContentType
		expected entity.ContentType
	}{
		{common.ContentTypeText, entity.ContentType_Text},
		{common.ContentTypeImage, entity.ContentType_Image},
		{common.ContentTypeAudio, entity.ContentType_Audio},
		{common.ContentTypeMultiPart, entity.ContentType_MultiPart},
		{common.ContentTypeVideo, entity.ContentType_Video},
		{"unknown", entity.ContentType_Text},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, evaluationset.ConvertContentTypeDTO2DO(tt.input))
		})
	}
}

func TestFillDatasetKeysFromSchema(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty schema JSON is no-op", func(t *testing.T) {
		t.Parallel()
		mappings := []*taskentity.EvaluateFieldMapping{
			{EvalSetName: gptr.Of("field_a")},
		}
		fillDatasetKeysFromSchema(ctx, mappings, "")
		assert.Nil(t, mappings[0].DatasetKey)
	})

	t.Run("invalid JSON is no-op", func(t *testing.T) {
		t.Parallel()
		mappings := []*taskentity.EvaluateFieldMapping{
			{EvalSetName: gptr.Of("field_a")},
		}
		fillDatasetKeysFromSchema(ctx, mappings, "not valid json")
		assert.Nil(t, mappings[0].DatasetKey)
	})

	t.Run("populates key from schema by name match", func(t *testing.T) {
		t.Parallel()
		schemaJSON := `[{"Key":"key_input","Name":"输入","ContentType":"text"},{"Key":"key_output","Name":"输出","ContentType":"text"}]`
		mappings := []*taskentity.EvaluateFieldMapping{
			{EvalSetName: gptr.Of("输入")},
			{EvalSetName: gptr.Of("输出")},
			{EvalSetName: gptr.Of("不存在的字段")},
		}
		fillDatasetKeysFromSchema(ctx, mappings, schemaJSON)
		assert.Equal(t, "key_input", *mappings[0].DatasetKey)
		assert.Equal(t, "key_output", *mappings[1].DatasetKey)
		assert.Nil(t, mappings[2].DatasetKey)
	})

	t.Run("overwrites existing DatasetKey", func(t *testing.T) {
		t.Parallel()
		schemaJSON := `[{"Key":"real_key","Name":"field_a","ContentType":"text"}]`
		oldKey := "stale_key"
		mappings := []*taskentity.EvaluateFieldMapping{
			{EvalSetName: gptr.Of("field_a"), DatasetKey: &oldKey},
		}
		fillDatasetKeysFromSchema(ctx, mappings, schemaJSON)
		assert.Equal(t, "real_key", *mappings[0].DatasetKey)
	})

	t.Run("skips schema entries with nil or empty key", func(t *testing.T) {
		t.Parallel()
		schemaJSON := `[{"Key":"","Name":"field_a","ContentType":"text"},{"Name":"field_b","ContentType":"text"}]`
		mappings := []*taskentity.EvaluateFieldMapping{
			{EvalSetName: gptr.Of("field_a")},
			{EvalSetName: gptr.Of("field_b")},
		}
		fillDatasetKeysFromSchema(ctx, mappings, schemaJSON)
		assert.Nil(t, mappings[0].DatasetKey)
		assert.Nil(t, mappings[1].DatasetKey)
	})

	t.Run("nil EvalSetName mapping is skipped", func(t *testing.T) {
		t.Parallel()
		schemaJSON := `[{"Key":"key_a","Name":"field_a","ContentType":"text"}]`
		mappings := []*taskentity.EvaluateFieldMapping{
			{EvalSetName: nil},
		}
		fillDatasetKeysFromSchema(ctx, mappings, schemaJSON)
		assert.Nil(t, mappings[0].DatasetKey)
	})
}

func TestBuildItem(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mapping := &taskentity.EvaluateFieldMapping{
		FieldSchema: &dataset.FieldSchema{
			Name:        gptr.Of("field_1"),
			ContentType: gptr.Of(common.ContentTypeText),
			TextSchema:  gptr.Of("{}"),
		},
		TraceFieldKey:      "Input",
		TraceFieldJsonpath: "",
		EvalSetName:        gptr.Of("field_1"),
		DatasetKey:         gptr.Of("my_key"),
	}

	span := &loop_span.Span{TraceID: "1234567890abcdef1234567890abcdef", SpanID: "feedbeeffeedbeef", Input: "hello"}
	data := buildItem(ctx, span, []*taskentity.EvaluateFieldMapping{mapping}, "run-1")
	assert.Len(t, data, 4)
	assert.Equal(t, "trace_id", data[0].GetKey())
	assert.Equal(t, "span_id", data[1].GetKey())
	assert.Equal(t, "run_id", data[2].GetKey())
	assert.Equal(t, "my_key", data[3].GetKey())
	assert.Equal(t, "field_1", data[3].GetName())
	assert.Equal(t, "hello", data[3].GetContent().GetText())

	// content error path should return nil
	mapping.FieldSchema.ContentType = gptr.Of(common.ContentTypeMultiPart)
	badSpan := &loop_span.Span{TraceID: span.TraceID, SpanID: span.SpanID, Input: "invalid json"}
	assert.Nil(t, buildItem(ctx, badSpan, []*taskentity.EvaluateFieldMapping{mapping}, "run-1"))

	// EvalSetName nil case should skip the field
	mapping.FieldSchema.ContentType = gptr.Of(common.ContentTypeText)
	mapping.EvalSetName = nil
	noName := buildItem(ctx, span, []*taskentity.EvaluateFieldMapping{mapping}, "run-1")
	assert.Len(t, noName, 3)
}

// Note: key-nil case cannot be safely tested because buildItem dereferences key

func TestBuildItems(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mapping := &taskentity.EvaluateFieldMapping{
		FieldSchema: &dataset.FieldSchema{
			Name:        gptr.Of("field_1"),
			ContentType: gptr.Of(common.ContentTypeText),
			TextSchema:  gptr.Of("{}"),
		},
		TraceFieldKey:      "Input",
		TraceFieldJsonpath: "",
		EvalSetName:        gptr.Of("field_1"),
	}

	goodSpan := &loop_span.Span{TraceID: "1234567890abcdef1234567890abcdef", SpanID: "deadc0debeefcafe", Input: "hello"}
	badSpan := &loop_span.Span{TraceID: goodSpan.TraceID, SpanID: "badbadbadbadbad", Input: "invalid"}

	mapping.FieldSchema.ContentType = gptr.Of(common.ContentTypeMultiPart)
	turns := buildItems(ctx, []*loop_span.Span{goodSpan, badSpan}, []*taskentity.EvaluateFieldMapping{mapping}, "run-1")
	assert.Empty(t, turns)

	mapping.FieldSchema.ContentType = gptr.Of(common.ContentTypeText)
	turns = buildItems(ctx, []*loop_span.Span{goodSpan, badSpan}, []*taskentity.EvaluateFieldMapping{mapping}, "run-1")
	assert.Len(t, turns, 2)
	for _, turn := range turns {
		assert.Equal(t, "run_id", turn.FieldDataList[2].GetKey())
	}

	// ensure spans returning nil items are skipped
	mapping.FieldSchema.ContentType = gptr.Of(common.ContentTypeMultiPart)
	turns = buildItems(ctx, []*loop_span.Span{badSpan}, []*taskentity.EvaluateFieldMapping{mapping}, "run-1")
	assert.Empty(t, turns)
}

func TestGetContentInfo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	c, code := entity.GetContentInfo(ctx, entity.ContentType_Text, "plain-text")
	assert.Equal(t, int64(0), code)
	assert.Equal(t, entity.ContentType_Text, c.ContentType)
	assert.Equal(t, "plain-text", c.Text)

	parts := []tracespec.ModelMessagePart{
		{
			Type: tracespec.ModelMessagePartTypeImage,
			ImageURL: &tracespec.ModelImageURL{
				Name: "image",
				URL:  "http://example.com/image.png",
			},
		},
		{
			Type: tracespec.ModelMessagePartTypeText,
			Text: "hello",
		},
		{
			Type: tracespec.ModelMessagePartTypeFile,
			Text: "file-content",
		},
	}
	payload, err := json.Marshal(parts)
	assert.NoError(t, err)

	c, code = entity.GetContentInfo(ctx, entity.ContentType_MultiPart, string(payload))
	assert.Equal(t, int64(0), code)
	assert.Equal(t, entity.ContentType_MultiPart, c.ContentType)
	assert.Len(t, c.MultiPart, 3)
	assert.Equal(t, entity.ContentType_Image, c.MultiPart[0].ContentType)
	assert.Equal(t, entity.ContentType_Text, c.MultiPart[1].ContentType)

	_, code = entity.GetContentInfo(ctx, entity.ContentType_MultiPart, "invalid json")
	assert.Equal(t, entity.DatasetErrorType_MismatchSchema, code)

	parts = []tracespec.ModelMessagePart{{Type: "unsupported"}}
	payload, err = json.Marshal(parts)
	assert.NoError(t, err)
	c, code = entity.GetContentInfo(ctx, entity.ContentType_MultiPart, string(payload))
	assert.Equal(t, entity.DatasetErrorType_MismatchSchema, code)
	assert.Nil(t, c)
}
