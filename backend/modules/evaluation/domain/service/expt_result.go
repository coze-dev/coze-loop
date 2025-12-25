// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate  mockgen -destination  ./mocks/expt_result.go  --package mocks . ExptResultService,ExptAggrResultService
type ExptResultService interface {
	MGetExperimentResult(ctx context.Context, param *entity.MGetExperimentResultParam) (*entity.MGetExperimentReportResult, error)
	// RecordItemRunLogs sync results from run_log table to result table
	RecordItemRunLogs(ctx context.Context, exptID, exptRunID, itemID, spaceID int64) ([]*entity.ExptTurnEvaluatorResultRef, error)
	GetExptItemTurnResults(ctx context.Context, exptID, itemID, spaceID int64, session *entity.Session) ([]*entity.ExptTurnResult, error)

	CreateStats(ctx context.Context, exptStats *entity.ExptStats, session *entity.Session) error
	GetStats(ctx context.Context, exptID, spaceID int64, session *entity.Session) (*entity.ExptStats, error)
	MGetStats(ctx context.Context, exptIDs []int64, spaceID int64, session *entity.Session) ([]*entity.ExptStats, error)
	CalculateStats(ctx context.Context, exptID, spaceID int64, session *entity.Session) (*entity.ExptCalculateStats, error)
	GetIncompleteTurns(ctx context.Context, exptID, spaceID int64, session *entity.Session) ([]*entity.ItemTurnID, error)

	ManualUpsertExptTurnResultFilter(ctx context.Context, spaceID, exptID int64, itemIDs []int64) error
	UpsertExptTurnResultFilter(ctx context.Context, spaceID, exptID int64, itemID []int64) error
	InsertExptTurnResultFilterKeyMappings(ctx context.Context, mappings []*entity.ExptTurnResultFilterKeyMapping) error
	CompareExptTurnResultFilters(ctx context.Context, spaceID, exptID int64, itemIDs []int64, retryTimes int32) error
}

type ExptAggrResultService interface {
	BatchGetExptAggrResultByExperimentIDs(ctx context.Context, spaceID int64, experimentIDs []int64) ([]*entity.ExptAggregateResult, error)
	// Calculate and persist aggregate results upon experiment completion.
	// Note: consider timing issues when updating scores.
	CreateExptAggrResult(ctx context.Context, spaceID, experimentID int64) error
	// Update aggregate results upon manual score correction.
	UpdateExptAggrResult(ctx context.Context, param *entity.UpdateExptAggrResultParam) error
	CreateAnnotationAggrResult(ctx context.Context, param *entity.CreateSpecificFieldAggrResultParam) error
	UpdateAnnotationAggrResult(ctx context.Context, param *entity.UpdateExptAggrResultParam) (err error)
	PublishExptAggrResultEvent(ctx context.Context, event *entity.AggrCalculateEvent, duration *time.Duration) error
}
