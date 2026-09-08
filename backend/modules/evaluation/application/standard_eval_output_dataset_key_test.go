// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func standardEvalTaskConfigItems(t *testing.T, content *exptpb.StandardEvalOutputContent) []map[string]any {
	t.Helper()
	var eval struct {
		TaskConfig struct {
			Items []map[string]any `json:"items"`
		} `json:"task_config"`
	}
	require.NoError(t, json.Unmarshal([]byte(content.GetText()), &eval))
	return eval.TaskConfig.Items
}

func TestBuildItemStandardEvalOutput_NormalizesPlatformDatasetKey(t *testing.T) {
	for _, tt := range []struct {
		name       string
		extKey     string
		payloadKey string
		want       string
	}{
		{name: "ext takes precedence", extKey: "dataset_new_runtime", payloadKey: "other_schema_v0_1", want: "dataset"},
		{name: "payload fallback", payloadKey: "dataset_schema_v0_1", want: "dataset"},
		{name: "stacked suffixes", extKey: "dataset_new_runtime_schema_v0_1", want: "dataset"},
		{name: "suffix in name", extKey: "dataset_new_runtime_part_schema_v0_1_data", want: "dataset_new_runtime_part_schema_v0_1_data"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
			item.Ext["dataset_key"] = tt.extKey
			payload := item.TurnResults[0].ExperimentResults[0].Payload
			payload.EvalSet.DatasetKey = tt.payloadKey

			got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.GetDatasetKey())
			items := standardEvalTaskConfigItems(t, got.GetEval())
			require.Len(t, items, 1)
			assert.Equal(t, tt.want, items[0]["dataset_key"])
			assert.Equal(t, "case-1", items[0]["item_key"])
			assert.Equal(t, tt.extKey, item.Ext["dataset_key"])
			assert.Equal(t, tt.payloadKey, payload.EvalSet.DatasetKey)
		})
	}
}

func TestBuildItemStandardEvalOutput_NormalizesReportedDatasetKeys(t *testing.T) {
	const reported = `{"task_config":{"items":[{"dataset_key":"dataset_new_runtime_schema_v0_1","item_key":"case_new_runtime"},{"dataset_key":"other_schema_v0_1_new_runtime","item_key":"other"},{"dataset_key":"name_new_runtime_part","item_key":"third"}],"mode":"reported"},"detail":{"eval_result":{"score":0.99}}}`
	for _, tt := range []struct {
		name   string
		fields map[string]string
	}{
		{
			name: "prefixed field overrides bare field",
			fields: map[string]string{
				"eval":        `{"task_config":{"items":[{"dataset_key":"ignored_schema_v0_1"}]}}`,
				"FORNAX_eval": reported,
			},
		},
		{name: "bare eval field", fields: map[string]string{"eval": reported}},
		{
			name: "legacy standard fields",
			fields: map[string]string{
				"source": `{"type":"fornax"}`,
				"rounds": `[]`,
				"output": `{}`,
				"eval":   reported,
			},
		},
		{
			name: "whole standard output",
			fields: map[string]string{
				consts.EvalTargetOutputFieldKeyActualOutput: `{"detail_id":"detail","source":"fornax","rounds":[],"output":{},"eval":` + reported + `}`,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
			item.Ext["dataset_key"] = "dataset_new_runtime"
			for field, value := range tt.fields {
				injectFornaxField(item, field, value)
			}
			fields := item.TurnResults[0].ExperimentResults[0].Payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields
			before, err := json.MarshalString(fields)
			require.NoError(t, err)

			got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
			require.NoError(t, err)
			assert.Equal(t, "dataset", got.GetDatasetKey())
			items := standardEvalTaskConfigItems(t, got.GetEval())
			require.Len(t, items, 3)
			assert.Equal(t, "dataset", items[0]["dataset_key"])
			assert.Equal(t, "other", items[1]["dataset_key"])
			assert.Equal(t, "name_new_runtime_part", items[2]["dataset_key"])
			assert.Equal(t, "case_new_runtime", items[0]["item_key"])
			var eval map[string]any
			require.NoError(t, json.Unmarshal([]byte(got.GetEval().GetText()), &eval))
			assert.Equal(t, "reported", eval["task_config"].(map[string]any)["mode"])
			assert.Equal(t, 0.99, eval["detail"].(map[string]any)["eval_result"].(map[string]any)["score"])
			after, err := json.MarshalString(fields)
			require.NoError(t, err)
			assert.JSONEq(t, before, after)
		})
	}
}

func TestBuildItemStandardEvalOutput_DatasetKeyNormalizationPreservesOpaqueEval(t *testing.T) {
	const preview = `{"task_config":{"items":[{"dataset_key":"dataset_new_runtime"}]}}`
	for _, tt := range []struct {
		name    string
		content *entity.Content
	}{
		{
			name: "omitted content",
			content: &entity.Content{
				Text:             gptr.Of(preview),
				ContentOmitted:   gptr.Of(true),
				FullContent:      &entity.ObjectStorage{URI: gptr.Of("eval:record:field:key")},
				FullContentBytes: gptr.Of(int32(4096)),
			},
		},
		{
			name: "full content reference",
			content: &entity.Content{
				Text:        gptr.Of(preview),
				FullContent: &entity.ObjectStorage{URI: gptr.Of("eval:record:field:key")},
			},
		},
		{name: "non JSON content", content: &entity.Content{Text: gptr.Of("dataset_new_runtime")}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			item := makeStandardEvalOutputReportResult(20, 30, 10, 1, 100).ItemResults[0]
			fields := item.TurnResults[0].ExperimentResults[0].Payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields
			fields["FORNAX_eval"] = tt.content

			got, err := buildItemStandardEvalOutput(context.Background(), item, standardEvalOutputBuildOptions{ExptID: 20})
			require.NoError(t, err)
			assert.Equal(t, contentToStandardEvalOutputContent(tt.content), got.GetEval())
		})
	}
}

func TestNormalizeStandardEvalDatasetKeys_PreservesOtherValues(t *testing.T) {
	content := &exptpb.StandardEvalOutputContent{
		Text: gptr.Of(`{"task_config":{"items":[{"dataset_key":"dataset_new_runtime","item_key":"item_schema_v0_1","external_id":9223372036854775807},null,"dataset_new_runtime",{"dataset_key":123}]},"detail":{"dataset_key":"detail_new_runtime"}}`),
	}
	require.NoError(t, normalizeStandardEvalDatasetKeys(content))
	assert.Contains(t, content.GetText(), "9223372036854775807")
	assert.JSONEq(t, `{"task_config":{"items":[{"dataset_key":"dataset","item_key":"item_schema_v0_1","external_id":9223372036854775807},null,"dataset_new_runtime",{"dataset_key":123}]},"detail":{"dataset_key":"detail_new_runtime"}}`, content.GetText())
}
