// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// Existing post-commit tests inject an already committed SQL result into the
// standard CompleteExpt boundary. This fixture has no resource/lease/business
// implementation; real SQL behavior is covered by the existing MySQL fixtures.
type completedFixtureRunRepo struct {
	repo.IExptRunLogRepo
	event *entity.ExptScheduleEvent
}

func (r *completedFixtureRunRepo) Get(context.Context, int64, int64) (*entity.ExptRunLog, error) {
	return &entity.ExptRunLog{ExptID: r.event.ExptID, ExptRunID: r.event.ExptRunID, SpaceID: r.event.SpaceID, Mode: int32(r.event.ExptRunMode), Status: int64(entity.ExptStatus_Success)}, nil
}

func completeCommittedFixture(m *ExptMangerImpl, ctx context.Context, event *entity.ExptScheduleEvent, finished *entity.Experiment, sources []*entity.ExptCurrentItemRunSource, opt *entity.CompleteExptOption) error {
	if m.runFinishRepo == nil {
		m.runFinishRepo = &finishRepoProbe{}
	}
	if m.runLogRepo == nil {
		m.runLogRepo = &completedFixtureRunRepo{event: event}
	}
	if m.mutex == nil && m.manualRetryLocker != nil {
		m.mutex = m.manualRetryLocker().WithHolder(legacyStaticHolderValue)
	}
	data := &completeData{result: &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishAlreadyFinished, Experiment: finished}, sources: sources, from: entity.ExptStatus_Processing}
	ctx = context.WithValue(ctx, completeDataKey{}, data)
	options := []entity.CompleteExptOptionFn{}
	if opt != nil {
		options = append(options, func(out *entity.CompleteExptOption) { *out = *opt })
	}
	return m.CompleteExpt(ctx, event.ExptID, &event.ExptRunID, event.SpaceID, event.Session, options...)
}

func completeNaturalFixture(m *ExptMangerImpl, ctx context.Context, event *entity.ExptScheduleEvent, finished *entity.Experiment) error {
	return completeCommittedFixture(m, ctx, event, finished, nil, nil)
}

func completeTerminationFixture(m *ExptMangerImpl, ctx context.Context, event *entity.ExptScheduleEvent, finished *entity.Experiment, sources []*entity.ExptCurrentItemRunSource, opt *entity.CompleteExptOption) error {
	return completeCommittedFixture(m, ctx, event, finished, sources, opt)
}

func completeCancelledFixture(m *ExptMangerImpl, ctx context.Context, event *entity.ExptScheduleEvent, finished *entity.Experiment, sources []*entity.ExptCurrentItemRunSource) error {
	return completeCommittedFixture(m, ctx, event, finished, sources, &entity.CompleteExptOption{NoCompleteItemTurn: true})
}

// Optional capability pass-through used only by lock-observing test wrappers.
type lifecycleLockValueReader interface {
	GetLifecycleLockValue(context.Context, string) (string, bool, error)
	GetLifecycleLockTTL(context.Context, string) (time.Duration, bool, error)
}
