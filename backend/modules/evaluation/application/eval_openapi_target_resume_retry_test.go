// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// targetResumeRetryFixture models the real ReportInvokeRecords contract: a record
// that is no longer AsyncInvoking is rejected, exactly like target_impl.go. The
// fake is stateful so a retry observably hits the terminal record written by the
// previous callback instead of merely reproducing the bug in the test.
type targetResumeRetryFixture struct {
	asyncRepo *repomocks.MockIEvalAsyncRepo
	targetSvc *servicemocks.MockIEvalTargetService
	publisher *eventmocks.MockExptEventPublisher
	configer  *mocks.MockIConfiger
	app       *EvalOpenAPIApplication
	record    *entity.EvalTargetRecord
	actx      *entity.EvalAsyncCtx
	writes    int
	writeErrs []error
	published []*entity.ExptItemEvalEvent
	publishFn func(*entity.ExptItemEvalEvent) error
}

func newTargetResumeRetryFixture(t *testing.T, ctrl *gomock.Controller, recordID, turnID int64) *targetResumeRetryFixture {
	t.Helper()
	f := &targetResumeRetryFixture{
		asyncRepo: repomocks.NewMockIEvalAsyncRepo(ctrl),
		targetSvc: servicemocks.NewMockIEvalTargetService(ctrl),
		publisher: eventmocks.NewMockExptEventPublisher(ctrl),
		configer:  mocks.NewMockIConfiger(ctrl),
		record:    &entity.EvalTargetRecord{ID: recordID, TurnID: turnID, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking)},
		actx: &entity.EvalAsyncCtx{
			Event:                   &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, RequestID: "original-rq"},
			ResumeReady:             true,
			AsyncUnixMS:             time.Now().UnixMilli() - 1000,
			EnableExtractTrajectory: gptr.Of(false),
		},
	}
	f.asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(recordID, 10)).Return(f.actx, nil).AnyTimes()
	f.targetSvc.EXPECT().GetRecordByID(gomock.Any(), int64(1), recordID).DoAndReturn(func(context.Context, int64, int64) (*entity.EvalTargetRecord, error) {
		return f.record, nil
	}).AnyTimes()
	// Mirrors ReportInvokeRecords: only AsyncInvoking can be written; anything
	// else is rejected before the caller has a chance to publish the resume.
	f.targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ReportTargetRecordParam) error {
		f.writes++
		if len(f.writeErrs) > 0 {
			err := f.writeErrs[0]
			f.writeErrs = f.writeErrs[1:]
			return err
		}
		if f.record.Status == nil || *f.record.Status != entity.EvalTargetRunStatusAsyncInvoking {
			return errors.New("unexpected target result status")
		}
		f.record.Status = gptr.Of(p.Status)
		return nil
	}).AnyTimes()
	f.configer.EXPECT().GetTargetTrajectoryConf(gomock.Any()).Return(&entity.TargetTrajectoryConf{}).AnyTimes()
	f.publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptItemEvalEvent, _ *time.Duration, modify func(*entity.ExptItemEvalEvent)) error {
		modify(event)
		f.published = append(f.published, event)
		if f.publishFn != nil {
			return f.publishFn(event)
		}
		return nil
	}).AnyTimes()
	f.app = &EvalOpenAPIApplication{asyncRepo: f.asyncRepo, targetSvc: f.targetSvc, publisher: f.publisher, configer: f.configer}
	return f
}

// A terminal write can succeed while its resume publish fails. The provider then
// retries — possibly with a flipped status — and ReportInvokeRecords rejects the
// now-terminal record. The retry must still publish the original resume source,
// otherwise the item is stranded until the zombie timeout.
func TestTargetTerminalRetryRepublishesResumeAfterPublishFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	f := newTargetResumeRetryFixture(t, ctrl, 100, 5)
	publishCause := errors.New("mq publish timeout")
	publishCalls := 0
	f.publishFn = func(*entity.ExptItemEvalEvent) error {
		publishCalls++
		if publishCalls == 1 {
			return publishCause
		}
		return nil
	}

	_, err := f.app.ReportEvalTargetInvokeResult_(context.Background(), newSuccessInvokeResultReq(1, 100))
	require.Error(t, err, "the first publish failure must be retryable")
	assert.Same(t, publishCause, err)

	_, err = f.app.ReportEvalTargetInvokeResult_(context.Background(), newFailedInvokeResultReq(1, 100, "provider retry flipped status"))
	require.NoError(t, err, "a retry that hits the terminal record must still republish resume")
	assert.Equal(t, 1, f.writes, "the terminal retry must not attempt another terminal write")
	require.Len(t, f.published, 2)
	for _, event := range f.published {
		assertCallbackResumeSource(t, event, "target", 100, 5)
		assert.True(t, event.AsyncReportTrigger)
		assert.Equal(t, "original-rq", event.RequestID)
	}
}

// Publish may actually succeed while the provider only observes a client
// timeout. Its retry is then a pure duplicate: republishing the same source is
// the only allowed side effect. The item source guard drops terminal-item
// events and turn refs reuse the existing record, so no target run repeats.
func TestTargetRetryAfterSuccessfulPublishOnlyRepublishesResume(t *testing.T) {
	ctrl := gomock.NewController(t)
	f := newTargetResumeRetryFixture(t, ctrl, 100, 5)

	_, err := f.app.ReportEvalTargetInvokeResult_(context.Background(), newSuccessInvokeResultReq(1, 100))
	require.NoError(t, err)
	_, err = f.app.ReportEvalTargetInvokeResult_(context.Background(), newSuccessInvokeResultReq(1, 100))
	require.NoError(t, err, "a duplicated provider retry must be acknowledged after republishing resume")
	assert.Equal(t, 1, f.writes)
	require.Len(t, f.published, 2)
	for _, event := range f.published {
		assertCallbackResumeSource(t, event, "target", 100, 5)
	}
}

// A genuine write error on an AsyncInvoking record is not a duplicate: it must
// stay fail-fast and must not publish a resume for a result that never landed.
func TestTargetGenuineWriteErrorIsNotSwallowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	f := newTargetResumeRetryFixture(t, ctrl, 100, 5)
	cause := errors.New("save target record failed")
	f.writeErrs = []error{cause}

	_, err := f.app.ReportEvalTargetInvokeResult_(context.Background(), newSuccessInvokeResultReq(1, 100))
	require.Error(t, err)
	assert.Same(t, cause, err)
	assert.Equal(t, 1, f.writes)
	assert.Empty(t, f.published)
}

// The recovery path must reuse the same source identity checks; a broken or
// still-invoking record is never turned into a guessed resume source.
func TestTargetTerminalRetrySourceValidationNotRelaxed(t *testing.T) {
	for _, sourceCase := range []string{"missing", "wrong record", "negative turn", "read failure"} {
		t.Run(sourceCase, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
			targetSvc := servicemocks.NewMockIEvalTargetService(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			configer := mocks.NewMockIConfiger(ctrl)
			actx := &entity.EvalAsyncCtx{Event: &entity.ExptItemEvalEvent{ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, RequestID: "original-rq"}}
			asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), "100").Return(actx, nil)
			record := &entity.EvalTargetRecord{ID: 100, TurnID: 5, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}
			var readErr error
			switch sourceCase {
			case "missing":
				record = nil
			case "wrong record":
				record.ID = 200
			case "negative turn":
				record.TurnID = -1
			case "read failure":
				readErr = errors.New("source storage unavailable")
			}
			targetSvc.EXPECT().GetRecordByID(gomock.Any(), int64(1), int64(100)).Return(record, readErr)
			targetSvc.EXPECT().ReportInvokeRecords(gomock.Any(), gomock.Any()).Times(0)
			publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			app := &EvalOpenAPIApplication{asyncRepo: asyncRepo, targetSvc: targetSvc, publisher: publisher, configer: configer}
			_, err := app.ReportEvalTargetInvokeResult_(context.Background(), newSuccessInvokeResultReq(1, 100))
			require.Error(t, err)
		})
	}
}
