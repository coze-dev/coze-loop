// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
	"testing"
)

type retryItemsLoaderFailurePorts struct {
	repo.IHookRepo
	repo.IHookRetryItemsTailRepo
	counted, finalized int
}

func (p *retryItemsLoaderFailurePorts) RecordRetryItemsSourceFailure(context.Context, entity.HookRunKey, string, string) (bool, error) {
	p.counted++
	return false, nil
}
func (p *retryItemsLoaderFailurePorts) BeginFinalize(context.Context, entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	p.finalized++
	return entity.HookStoreResult{}, nil
}

func TestHookRetryItemsRealLoaderIOClassification(t *testing.T) {
	for _, kind := range []string{"version_io", "batch_io", "schema_io", "resolver_io", "missing_version", "wrong_version", "missing_schema", "unresolved_content", "known_content_error"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			f := exactLoaderFixture()
			if kind == "batch_io" {
				f = newLoaderFixture()
			}
			api, err := NewHookFrozenPlanLoader(f.deps)
			require.NoError(t, err)
			loader := api.(retryItemsCandidateLoader)
			_, err = loader.LoadCandidates(ctx, f.input, f.plans.page)
			require.NoError(t, err)
			ioErr := errors.New("private transport diagnostic must not escape")
			fallback := entity.ErrHookFrozenItemUnavailable
			transient := true
			switch kind {
			case "version_io", "batch_io":
				f.items.err = ioErr
			case "schema_io":
				f.versions.err = ioErr
				fallback = entity.ErrHookFrozenSchemaInvalid
			case "missing_version":
				f.items.version = nil
				transient = false
			case "wrong_version":
				f.items.version.ItemVersionID++
				transient = false
			case "missing_schema":
				f.versions.value.EvaluationSetSchema = nil
				fallback = entity.ErrHookFrozenSchemaInvalid
				transient = false
			default:
				fallback = entity.ErrHookFrozenContentUnavailable
				f.items.version.Turns[0].FieldDataList[0].Content.ContentOmitted = gptr.Of(true)
				f.deps.Resolver = &loaderResolver{fn: func(context.Context, entity.HookPlanItem, int64, string, *entity.Content) (*entity.Content, error) {
					if kind == "resolver_io" {
						return nil, ioErr
					}
					if kind == "known_content_error" {
						return nil, entity.ErrHookFrozenContentUnavailable
					}
					return nil, nil
				}}
				transient = kind == "resolver_io"
				api, err = NewHookFrozenPlanLoader(f.deps)
				require.NoError(t, err)
				loader = api.(retryItemsCandidateLoader)
			}
			page, sourceErr := loader.LoadCandidates(ctx, f.input, f.plans.page)
			require.Error(t, sourceErr)
			require.Nil(t, page)
			require.NotContains(t, sourceErr.Error(), "private transport")
			ports := &retryItemsLoaderFailurePorts{}
			s := &ExptSchedulerImpl{hookRuns: ports, hookSchedulerScope: f.input.ExecutionScope}
			run := &entity.HookStoredRun{State: entity.HookRunState{Key: f.input.Key}, Version: 7, PlanCursor: "accepted-tail"}
			decision := s.retryItemsSourceError(ctx, run, ports, sourceErr)
			if transient {
				require.False(t, permanentHookInitializationError(sourceErr))
				require.ErrorIs(t, decision, ErrHookPlanSourceRetry)
				require.Equal(t, 1, ports.counted)
				require.Zero(t, ports.finalized)
			} else {
				require.True(t, permanentHookInitializationError(sourceErr))
				require.ErrorIs(t, decision, ErrHookPlanPreparationFailed)
				require.Zero(t, ports.counted)
				require.Equal(t, 1, ports.finalized)
			}
			_, legacyErr := api.LoadPage(ctx, f.input)
			require.ErrorIs(t, legacyErr, fallback, "published LoadPage retains the old four-mode error contract")
		})
	}
}
