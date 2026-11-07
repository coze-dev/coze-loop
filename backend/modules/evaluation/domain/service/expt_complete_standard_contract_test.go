// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Standard CompleteExpt treats template/aggregation/notification failures as
// best effort and honors its NoAggrCalculate option after current SQL closure.
// A separate Finish business chain must not override these established duties.
func TestCompleteStandardContractHonorsOptionsAndBestEffort(t *testing.T) {
	for _, fault := range []string{"no_aggr", "template"} {
		t.Run(fault, func(t *testing.T) {
			p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
			p.publishSuccess(t, false)
			f := p.f
			ctrl := gomock.NewController(t)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			publisher.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			p.manager.publisher = publisher
			aggr := svcmocks.NewMockExptAggrResultService(ctrl)
			p.manager.exptAggrResultService = aggr
			var opts []entity.CompleteExptOptionFn
			if fault == "no_aggr" {
				opts = append(opts, entity.NoAggrCalculate())
				// Any aggregate publication is an unexpected call.
			} else {
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("expt_template_id", f.base+700).Error)
				template := svcmocks.NewMockIExptTemplateManager(ctrl)
				template.EXPECT().UpdateExptInfo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("template unavailable"))
				p.manager.templateManager = template
				aggr.EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			}
			require.NoError(t, p.manager.CompleteExpt(context.Background(), f.expt, &f.run, f.space, p.event.Session, opts...))
			current, err := p.manager.exptRepo.GetByID(context.Background(), f.expt, f.space)
			require.NoError(t, err)
			require.Equal(t, f.run, current.LatestRunID)
			require.Equal(t, entity.ExptStatus_Success, current.Status)
			require.NotNil(t, current.EndAt)
			require.Equal(t, int32(1), p.resources.Load())
			require.Equal(t, int32(1), p.notifications.Load())
			locked, _, err := p.manager.mutex.BackoffLockWithValue(context.Background(), p.manager.makeExptMutexLockKey(f.expt), "next-run", time.Minute, time.Millisecond)
			require.NoError(t, err)
			require.True(t, locked, "standard completion must release original admission after its duties")
		})
	}
}
