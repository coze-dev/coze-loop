// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination mocks/expt_event_publisher_mock.go -package mocks . ExptEventPublisher
type ExptEventPublisher interface {
	PublishExptScheduleEvent(ctx context.Context, event *entity.ExptScheduleEvent, duration *time.Duration) error
	PublishExptRecordEvalEvent(ctx context.Context, event *entity.ExptItemEvalEvent, duration *time.Duration) error
	BatchPublishExptRecordEvalEvent(ctx context.Context, events []*entity.ExptItemEvalEvent, duration *time.Duration) error
	PublishExptAggrCalculateEvent(ctx context.Context, events []*entity.AggrCalculateEvent, duration *time.Duration) error
	PublishExptOnlineEvalResult(ctx context.Context, events *entity.OnlineExptEvalResultEvent, duration *time.Duration) error
	PublishExptTurnResultFilterEvent(ctx context.Context, event *entity.ExptTurnResultFilterEvent, duration *time.Duration) error
}

//go:generate mockgen -destination mocks/evaluator_event_publisher_mock.go -package mocks . EvaluatorEventPublisher
type EvaluatorEventPublisher interface {
	PublishEvaluatorRecordCorrection(ctx context.Context, events *entity.EvaluatorRecordCorrectionEvent, duration *time.Duration) error
}
