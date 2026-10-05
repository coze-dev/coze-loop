// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"bytes"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/stretchr/testify/require"
)

func TestHookProjectionResultHTMLDoesNotExpand(t *testing.T) {
	want := map[string]string{"key": strings.Repeat("<>&", 2000), "context": `{"creator":"业务","token":"literal"}`, "creator": "原值"}
	b, err := hookExecutionEncodeResult(want)
	require.NoError(t, err)
	body := append(append([]byte(`{"status":"succeeded","result":`), b...), '}')
	r, outcome := hookinfra.DecodeResponse(200, "application/json", bytes.NewReader(body))
	require.Equal(t, entity.HookSucceeded, outcome.Code)
	require.Equal(t, want, r.GetResult_())
}

func TestHookProjectionResultReservesSummaryEnvelope(t *testing.T) {
	// Map alone fits 32KiB; the complete succeeded response does not.
	want := map[string]string{"a": strings.Repeat("a", 8192), "b": strings.Repeat("b", 8192), "c": strings.Repeat("c", 8192), "d": strings.Repeat("d", 8150)}
	_, err := hookExecutionEncodeResult(want)
	require.Error(t, err)
}

func TestHookProjectionResultUnicodeSeparatorsDoNotExpand(t *testing.T) {
	value := strings.Repeat("\u2028\u2029", 1000)
	want := map[string]string{"a": value, "b": value, "c": value, "d": value, "literal": `\u2028`}
	b, err := hookExecutionEncodeResult(want)
	require.NoError(t, err)
	body := append(append([]byte(`{"status":"succeeded","result":`), b...), '}')
	r, outcome := hookinfra.DecodeResponse(200, "application/json", bytes.NewReader(body))
	require.Equal(t, entity.HookSucceeded, outcome.Code)
	require.Equal(t, want, r.GetResult_())
}
