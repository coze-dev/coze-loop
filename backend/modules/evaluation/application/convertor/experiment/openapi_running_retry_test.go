// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"reflect"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/js_conv"
)

func TestOpenAPIRunningRetryLatestRun(t *testing.T) {
	assert.Nil(t, DomainExperimentDTO2OpenAPI(nil))
	assert.Nil(t, OpenAPIExptDO2DTO(nil))
	for _, runID := range []int64{0, 9007199254740993, 9223372036854775807} {
		for _, dto := range []interface{}{
			DomainExperimentDTO2OpenAPI(&domainexpt.Experiment{ID: gptr.Of(int64(7)), LatestRunID: gptr.Of(runID)}),
			OpenAPIExptDO2DTO(&entity.Experiment{ID: 7, LatestRunID: runID}),
		} {
			field := reflect.ValueOf(dto).Elem().FieldByName("LatestRunID")
			require.True(t, field.IsValid(), "OpenAPI experiment must expose the authoritative latest_run_id")
			require.False(t, field.IsNil())
			assert.Equal(t, runID, field.Elem().Int())
		}
	}
}

func TestOpenAPIRunningRetryContractRoundTrip(t *testing.T) {
	t.Run("request JSON and Thrift preserve identity", func(t *testing.T) {
		input := `{"workspace_id":"9","experiment_id":"7","retry_mode":"retry_target_items","item_ids":["9223372036854775806"],"expected_run_id":"9223372036854775807","request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05"}`
		var req openapi.RetryExperimentOApiRequest
		require.NoError(t, js_conv.GetUnmarshaler()([]byte(input), &req))
		buf := make([]byte, req.BLength())
		require.Equal(t, len(buf), req.FastWrite(buf))
		var decoded openapi.RetryExperimentOApiRequest
		_, err := decoded.FastRead(buf)
		require.NoError(t, err)
		body, err := js_conv.GetMarshaler()(&decoded)
		require.NoError(t, err)
		assert.JSONEq(t, input, string(body))
	})
	for _, input := range []string{
		`{"run_id":"9223372036854775807","request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05","accepted_count":0,"rejected_items":[],"snapshot_at_ms":"1793275200123"}`,
		`{"run_id":"9223372036854775807","request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05","accepted_count":1,"rejected_items":[{"item_id":"9223372036854775806","reason":"busy"}],"snapshot_at_ms":"1793275200123"}`,
	} {
		t.Run(input, func(t *testing.T) {
			var receipt openapi.RetryExperimentOpenAPIData
			require.NoError(t, js_conv.GetUnmarshaler()([]byte(input), &receipt))
			buf := make([]byte, receipt.BLength())
			require.Equal(t, len(buf), receipt.FastWrite(buf))
			var decoded openapi.RetryExperimentOpenAPIData
			_, err := decoded.FastRead(buf)
			require.NoError(t, err)
			body, err := js_conv.GetMarshaler()(&decoded)
			require.NoError(t, err)
			assert.JSONEq(t, input, string(body))
		})
	}
}
