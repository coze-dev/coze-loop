// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

var projectionKey = entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}

type projectionNoIdentityContext struct{ context.Context }

func (projectionNoIdentityContext) Value(any) any { panic("projection must not inspect identity") }

func TestCompletionProjectorCopiesAllBusinessStrings(t *testing.T) {
	p := NewCompletionProjector()
	want := map[string]string{"creator": "business-user", "key": "业务键", "context": `{"x":1}`, "token": "word", "summary": "<>&\u2028  😀"}
	in := &spi.InvokeExperimentHookResponse{Status: gptr.Of("succeeded"), Result_: want}
	out, err := p.Project(projectionNoIdentityContext{context.Background()}, projectionKey, entity.HookPhaseBefore, in)
	require.NoError(t, err)
	require.Equal(t, want, out)
	out["creator"] = "changed"
	require.Equal(t, "business-user", in.Result_["creator"])
	in.Result_["token"] = "changed"
	require.Equal(t, "word", out["token"])
	for _, input := range []map[string]string{nil, {}} {
		got, err := p.Project(context.Background(), projectionKey, entity.HookPhaseAfter, &spi.InvokeExperimentHookResponse{Status: gptr.Of("succeeded"), Result_: input})
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
	}
}

func TestCompletionProjectorCopiesBusinessErrorAndDefaults(t *testing.T) {
	p := NewCompletionProjector()
	for _, code := range []*string{nil, gptr.Of("HTTP_ERROR"), gptr.Of("HOOK_SECURITY_ERROR"), gptr.Of(strings.Repeat("😀", 32))} {
		in := &spi.InvokeExperimentHookResponse{Status: gptr.Of("failed"), Error: &spi.HookError{Code: code, Message: gptr.Of("请稍后重试😀"), Retryable: gptr.Of(true)}}
		got, err := p.ProjectError(projectionNoIdentityContext{context.Background()}, projectionKey, entity.HookPhaseBefore, in)
		require.NoError(t, err)
		wantCode := "HOOK_FAILED"
		if code != nil {
			wantCode = *code
		}
		require.Equal(t, wantCode, got.GetCode())
		require.Equal(t, "请稍后重试😀", got.GetMessage())
		require.True(t, got.GetRetryable())
		*got.Code = "changed"
		*got.Message = "changed"
		*got.Retryable = false
		require.Equal(t, "请稍后重试😀", in.Error.GetMessage())
		require.True(t, in.Error.GetRetryable())
		if code == nil {
			require.Nil(t, in.Error.Code)
		} else {
			require.Equal(t, wantCode, in.Error.GetCode())
		}
	}
}

func TestCompletionProjectorRejectsInvalidScopeAndStatus(t *testing.T) {
	p := NewCompletionProjector()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name  string
		ctx   context.Context
		key   entity.HookRunKey
		phase entity.HookPhase
	}{
		{"nil context", nil, projectionKey, entity.HookPhaseBefore}, {"canceled", ctx, projectionKey, entity.HookPhaseBefore},
		{"invalid key", context.Background(), entity.HookRunKey{}, entity.HookPhaseBefore}, {"phase", context.Background(), projectionKey, "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Project(tc.ctx, tc.key, tc.phase, &spi.InvokeExperimentHookResponse{Status: gptr.Of("succeeded")})
			require.Error(t, err)
			_, err = p.ProjectError(tc.ctx, tc.key, tc.phase, &spi.InvokeExperimentHookResponse{Status: gptr.Of("failed"), Error: &spi.HookError{Message: gptr.Of("m")}})
			require.Error(t, err)
		})
	}
	for _, r := range []*spi.InvokeExperimentHookResponse{nil, {}, {Status: gptr.Of("unknown")}, {Status: gptr.Of("failed")}, {Status: gptr.Of("succeeded"), Error: &spi.HookError{Message: gptr.Of("m")}}} {
		_, err := p.Project(context.Background(), projectionKey, entity.HookPhaseBefore, r)
		require.Error(t, err)
	}
	for _, r := range []*spi.InvokeExperimentHookResponse{nil, {}, {Status: gptr.Of("succeeded")}, {Status: gptr.Of("failed")}, {Status: gptr.Of("failed"), Result_: map[string]string{}, Error: &spi.HookError{Message: gptr.Of("m")}}} {
		_, err := p.ProjectError(context.Background(), projectionKey, entity.HookPhaseBefore, r)
		require.Error(t, err)
	}
}

func TestCompletionProjectorStringAndCountLimits(t *testing.T) {
	p := NewCompletionProjector()
	for _, m := range []map[string]string{{"": "x"}, {strings.Repeat("k", 129): "x"}, {"k": strings.Repeat("v", 8193)}, {"k": string([]byte{255})}, {string([]byte{255}): "v"}} {
		_, err := p.Project(context.Background(), projectionKey, entity.HookPhaseBefore, &spi.InvokeExperimentHookResponse{Status: gptr.Of("succeeded"), Result_: m})
		require.Error(t, err)
	}
	m := map[string]string{}
	for i := 0; i < 128; i++ {
		m[fmt.Sprint(i)] = "v"
	}
	_, err := p.Project(context.Background(), projectionKey, entity.HookPhaseBefore, &spi.InvokeExperimentHookResponse{Status: gptr.Of("succeeded"), Result_: m})
	require.NoError(t, err)
	m["extra"] = "v"
	_, err = p.Project(context.Background(), projectionKey, entity.HookPhaseBefore, &spi.InvokeExperimentHookResponse{Status: gptr.Of("succeeded"), Result_: m})
	require.Error(t, err)
	_, err = p.Project(context.Background(), projectionKey, entity.HookPhaseBefore, &spi.InvokeExperimentHookResponse{Status: gptr.Of("succeeded"), Result_: map[string]string{strings.Repeat("k", 128): strings.Repeat("v", 8192)}})
	require.NoError(t, err)
	for _, e := range []*spi.HookError{{Message: nil}, {Message: gptr.Of(" ")}, {Message: gptr.Of(strings.Repeat("中", 683))}, {Message: gptr.Of(string([]byte{255}))}, {Code: gptr.Of(""), Message: gptr.Of("m")}, {Code: gptr.Of(strings.Repeat("😀", 33)), Message: gptr.Of("m")}, {Code: gptr.Of(string([]byte{255})), Message: gptr.Of("m")}} {
		_, err := p.ProjectError(context.Background(), projectionKey, entity.HookPhaseAfter, &spi.InvokeExperimentHookResponse{Status: gptr.Of("failed"), Error: e})
		require.Error(t, err)
	}
}

func TestCompletionProjectorFullResponseBoundary(t *testing.T) {
	// The input JSON is hand-sized independently of the production serializer.
	const empty = `{"status":"succeeded","result":{"a":"","b":"","c":"","d":""}}`
	for _, delta := range []int{0, 1} {
		last := 32768 - len(empty) - 3*8192 + delta
		body := fmt.Sprintf(`{"status":"succeeded","result":{"a":"%s","b":"%s","c":"%s","d":"%s"}}`, strings.Repeat("<", 8192), strings.Repeat("&", 8192), strings.Repeat(">", 8192), strings.Repeat("x", last))
		require.Len(t, body, 32768+delta)
		r, outcome := DecodeResponse(200, "application/json", strings.NewReader(body))
		if delta == 1 {
			require.Equal(t, entity.HookResponseTooLarge, outcome.Code)
			continue
		}
		require.Equal(t, entity.HookSucceeded, outcome.Code)
		got, err := NewCompletionProjector().Project(context.Background(), projectionKey, entity.HookPhaseBefore, r)
		require.NoError(t, err)
		encoded, err := entity.EncodeHookDisplayResult(got)
		require.NoError(t, err)
		roundtrip, outcome := DecodeResponse(200, "application/json", strings.NewReader(`{"status":"succeeded","result":`+string(encoded)+`}`))
		require.Equal(t, entity.HookSucceeded, outcome.Code)
		require.Equal(t, r.GetResult_(), roundtrip.GetResult_())
	}
}
