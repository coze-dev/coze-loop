// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestResponseDefaultsAndStrings(t *testing.T) {
	for _, raw := range []string{`{"status":"succeeded"}`, `{"status":"succeeded","result":null,"error":null}`} {
		response, outcome := DecodeResponse(200, "application/json; charset=utf-8", strings.NewReader(raw))
		require.Equal(t, entity.HookSucceeded, outcome.Code)
		require.NotNil(t, response.Result_)
		require.Empty(t, response.Result_)
	}
	response, outcome := DecodeResponse(200, "application/json", strings.NewReader(`{"status":"failed","result":null,"error":{"message":"failed","code":null,"retryable":null}}`))
	require.Equal(t, entity.HookFailed, outcome.Code)
	require.False(t, outcome.Retryable)
	require.Equal(t, "HOOK_FAILED", *response.Error.Code)
	require.False(t, *response.Error.Retryable)
	response, outcome = DecodeResponse(200, "application/json", strings.NewReader(`{"status":"failed","error":{"code":"BUSY","message":"try later","retryable":true}}`))
	require.True(t, outcome.Retryable)
	require.Equal(t, "BUSY", *response.Error.Code)
	response, outcome = DecodeResponse(200, "application/json", strings.NewReader(`{"status":"succeeded","result":{"action":"skipped","data":"{\"n\":9007199254740993}"}}`))
	require.Equal(t, entity.HookSucceeded, outcome.Code)
	require.Equal(t, `{"n":9007199254740993}`, response.Result_["data"])
}

func TestResponseUsesOnlyExactProtocolFieldNames(t *testing.T) {
	for _, raw := range []string{`{"Status":"succeeded"}`, `{"status":"failed","error":{"Message":"missing"}}`, `{"status":"bad","Status":"succeeded"}`} {
		_, outcome := DecodeResponse(200, "application/json", strings.NewReader(raw))
		require.Equal(t, entity.HookProtocolError, outcome.Code)
	}
	response, outcome := DecodeResponse(200, "application/json", strings.NewReader(`{"status":"succeeded","Result":{"x":null},"Error":{"message":"ignored"}}`))
	require.Equal(t, entity.HookSucceeded, outcome.Code)
	require.Empty(t, response.Result_)
	response, outcome = DecodeResponse(200, "application/json", strings.NewReader(`{"status":"failed","error":{"message":"failed","Retryable":true,"Code":"IGNORED"}}`))
	require.Equal(t, entity.HookFailed, outcome.Code)
	require.False(t, outcome.Retryable)
	require.Equal(t, "HOOK_FAILED", *response.Error.Code)
}

func TestResponseProtocolRejections(t *testing.T) {
	for _, raw := range []string{
		"", "null", "[]", "{}", `{"status":null}`, `{"status":true}`, `{"status":"pending"}`,
		`{"status":"succeeded","result":{"x":null}}`, `{"status":"succeeded","result":{"x":1}}`,
		`{"status":"succeeded","result":{"x":true}}`, `{"status":"succeeded","result":{"x":{}}}`,
		`{"status":"succeeded","result":[]}`, `{"status":"succeeded","error":{}}`,
		`{"status":"failed","result":{},"error":{"message":"x"}}`,
		`{"status":"failed"}`, `{"status":"failed","error":null}`, `{"status":"failed","error":{"message":""}}`,
		`{"status":"failed","error":{"message":null}}`, `{"status":"failed","error":{"message":1}}`,
		`{"status":"failed","error":{"message":"x","code":""}}`, `{"status":"failed","error":{"message":"x","retryable":"true"}}`,
		`{"status":"failed","error":{"message":"x","retryable":1}}`,
		`{"status":"succeeded","status":"succeeded"}`, `{"status":"succeeded","unknown":{"a":1,"\u0061":2}}`,
		`{"status":"succeeded","unknown":[{"a":1,"a":2}]}`, `{"status":"succeeded"} {}`,
		"{\"status\":\"succeeded\",\"unknown\":\"\xff\"}",
		`{"status":"succeeded","result":{"":"x"}}`,
		`{"status":"succeeded","result":{"` + strings.Repeat("a", 129) + `":"x"}}`,
		`{"status":"succeeded","result":{"x":"` + strings.Repeat("a", 8193) + `"}}`,
		`{"status":"failed","error":{"message":"` + strings.Repeat("a", 2049) + `"}}`,
		`{"status":"failed","error":{"message":"x","code":"` + strings.Repeat("a", 129) + `"}}`,
	} {
		response, outcome := DecodeResponse(200, "application/json", strings.NewReader(raw))
		require.Nil(t, response, raw)
		require.Equal(t, entity.HookProtocolError, outcome.Code, raw)
		require.False(t, outcome.Retryable)
	}
	for _, contentType := range []string{"", "text/plain", "application/problem+json", "application/json; bad"} {
		_, outcome := DecodeResponse(200, contentType, strings.NewReader(`{"status":"succeeded"}`))
		require.Equal(t, entity.HookProtocolError, outcome.Code)
	}
}

func TestResponseSizeMapLimitsAndUnknownFields(t *testing.T) {
	prefix := `{"status":"succeeded","padding":""}`
	for _, size := range []int{32768, 32769} {
		raw := `{"status":"succeeded","padding":"` + strings.Repeat("a", size-len(prefix)) + `"}`
		_, outcome := DecodeResponse(200, "application/json", strings.NewReader(raw))
		want := entity.HookSucceeded
		if size > 32768 {
			want = entity.HookResponseTooLarge
		}
		require.Equal(t, want, outcome.Code)
	}
	for _, count := range []int{128, 129} {
		pairs := make([]string, count)
		for i := range pairs {
			pairs[i] = fmt.Sprintf(`"k%d":"v"`, i)
		}
		_, outcome := DecodeResponse(200, "application/json", strings.NewReader(`{"status":"succeeded","result":{`+strings.Join(pairs, ",")+`}}`))
		want := entity.HookSucceeded
		if count > 128 {
			want = entity.HookProtocolError
		}
		require.Equal(t, want, outcome.Code)
	}
	unknown := make([]string, 300)
	for i := range unknown {
		unknown[i] = fmt.Sprintf(`"k%d":null`, i)
	}
	for _, tc := range []struct {
		raw  string
		code entity.HookOutcomeCode
	}{
		{`{"status":"succeeded","unknown":{` + strings.Join(unknown, ",") + `}}`, entity.HookSucceeded},
		{`{"status":"succeeded","unknown":` + strings.Repeat(`[`, 20) + `0` + strings.Repeat(`]`, 20) + `}`, entity.HookSucceeded},
		{`{"status":"succeeded","result":{"` + strings.Repeat("k", 128) + `":"` + strings.Repeat("v", 8192) + `"}}`, entity.HookSucceeded},
		{`{"status":"failed","error":{"message":"` + strings.Repeat("m", 2048) + `","code":"` + strings.Repeat("c", 128) + `"}}`, entity.HookFailed},
	} {
		_, outcome := DecodeResponse(200, "application/json", strings.NewReader(tc.raw))
		require.Equal(t, tc.code, outcome.Code)
	}
}

type countReader struct{ read int }

func (r *countReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	r.read += len(p)
	return len(p), nil
}

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, errors.New("sensitive raw transport error") }

func TestHTTPClassificationPrecedesBodyAndReadsAreBounded(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   entity.HookOutcomeCode
		retry  bool
	}{
		{202, entity.HookResultNotFinal, false}, {204, entity.HookProtocolError, false}, {301, entity.HookHTTPRedirectError, false},
		{400, entity.HookHTTPError, false}, {401, entity.HookHTTPError, false}, {403, entity.HookHTTPError, false}, {409, entity.HookHTTPError, false},
		{408, entity.HookHTTPError, true}, {429, entity.HookHTTPError, true}, {500, entity.HookHTTPError, true}, {599, entity.HookHTTPError, true},
	} {
		reader := &countReader{}
		response, outcome := DecodeResponse(tc.status, "text/plain", reader)
		require.Nil(t, response)
		require.Equal(t, entity.HookOutcome{Code: tc.code, HTTPStatus: tc.status, Retryable: tc.retry}, outcome)
		require.LessOrEqual(t, reader.read, 32768)
		_, outcome = DecodeResponse(tc.status, "application/json", failedReader{})
		require.Equal(t, entity.HookOutcome{Code: tc.code, HTTPStatus: tc.status, Retryable: tc.retry}, outcome)
	}
	reader := &countReader{}
	_, outcome := DecodeResponse(200, "application/json", reader)
	require.Equal(t, entity.HookResponseTooLarge, outcome.Code)
	require.Equal(t, 32769, reader.read)
	_, outcome = DecodeResponse(200, "application/json", failedReader{})
	require.Equal(t, entity.HookTransportError, outcome.Code)
	_, outcome = DecodeResponse(200, "application/json", nil)
	require.Equal(t, entity.HookProtocolError, outcome.Code)
}
