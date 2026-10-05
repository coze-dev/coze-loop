// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func projectionExecutor(t *testing.T, f *executorFixture, status int, body string) *service.HookAttemptExecutor {
	t.Helper()
	return f.executor(t, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult {
		r, outcome := hookinfra.DecodeResponse(status, "application/json", strings.NewReader(body))
		return entity.HookTransportResult{Response: r, Outcome: outcome, LocalCompletedAt: time.Now()}
	}), hookinfra.NewCompletionProjector())
}

func projectionSummary(t *testing.T, f *executorFixture) entity.HookRunSummary {
	t.Helper()
	m, err := experiment.NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), []entity.HookRunKey{f.input.Key})
	require.NoError(t, err)
	require.NotNil(t, m[f.input.Key])
	if f.input.Phase == entity.HookPhaseAfter {
		return m[f.input.Key].After
	}
	return m[f.input.Key].Before
}

func TestHookProjectionExecutorArbitraryBusinessMap(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	e := projectionExecutor(t, f, 200, `{"status":"succeeded","result":{"creator":"业务作者","key":"literal-key","context":"{\"x\":true}","token":"词元","custom":"<>&😀"}}`)
	_, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	want := map[string]string{"creator": "业务作者", "key": "literal-key", "context": `{"x":true}`, "token": "词元", "custom": "<>&😀"}
	summary := projectionSummary(t, f)
	require.Equal(t, entity.HookOperationSucceeded, summary.Status)
	require.Equal(t, want, summary.Response.GetResult_())
	require.Nil(t, summary.Error)
}

func TestHookProjectionExecutorBusinessErrorDisplay(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	e := projectionExecutor(t, f, 200, `{"status":"failed","error":{"code":"资源未就绪😀","message":"请稍后重试","retryable":false}}`)
	out, err := e.Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.True(t, out.Effects.BeginFinalize)
	row := f.operation(t)
	require.Equal(t, "资源未就绪😀", gptr.Indirect(row.ErrorCode))
	require.Equal(t, "请稍后重试", gptr.Indirect(row.ErrorMessage))
	summary := projectionSummary(t, f)
	require.Equal(t, "资源未就绪😀", summary.Error.GetCode())
	require.Equal(t, "请稍后重试", summary.Error.GetMessage())
	var audit model.ExptLifecycleHookAttempt
	require.NoError(t, f.sql.Where("operation_id=?", f.input.OperationID).First(&audit).Error)
	require.Equal(t, "HOOK_FAILED", gptr.Indirect(audit.ResultCategory))
	require.Contains(t, string(gptr.Indirect(audit.ErrorRedacted)), "completion_time")
	require.NotContains(t, string(gptr.Indirect(audit.ErrorRedacted)), "请稍后")
}
