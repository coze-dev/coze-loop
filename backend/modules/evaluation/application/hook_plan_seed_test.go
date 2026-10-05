// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type seedAppManager struct {
	service.IExptManager
	calls   []string
	raw     string
	mode    entity.ExptRunMode
	seedErr error
}

func (m *seedAppManager) LogRun(context.Context, int64, int64, entity.ExptRunMode, int64, []int64, *entity.Session) error {
	m.calls = append(m.calls, "legacy")
	return nil
}
func (m *seedAppManager) Run(_ context.Context, _, _, _ int64, _ int, _ *entity.Session, mode entity.ExptRunMode, ext map[string]string) error {
	m.calls = append(m.calls, "publish")
	m.mode = mode
	return errors.New("MQ unavailable")
}

type seedCapAppManager struct{ *seedAppManager }

func (m seedCapAppManager) LogRunWithPlanSeed(_ context.Context, _, _ int64, _ entity.ExptRunMode, _ int64, raw string, _ *entity.Session) error {
	m.raw = raw
	m.calls = append(m.calls, "seed")
	return m.seedErr
}

func TestHookPlanSeedApplicationRawInputAndInitializationFailure(t *testing.T) {
	for _, ext := range []map[string]string{nil, {}, {"__item_ids": ""}, {"__item_ids": "[]"}, {"__item_ids": "null"}, {"__item_ids": "{invalid"}} {
		ids := idmocks.NewMockIIDGenerator(gomock.NewController(t))
		ids.EXPECT().GenID(gomock.Any()).Return(int64(30), nil)
		failure := errors.New("initialization failure")
		m := &seedAppManager{seedErr: failure}
		app := &experimentApplication{manager: seedCapAppManager{m}, idgen: ids}
		_, err := app.RunExperiment(context.Background(), &exptpb.RunExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20)), TrialRunItemCount: gptr.Of(int64(99)), Ext: ext})
		require.ErrorIs(t, err, failure)
		require.Equal(t, []string{"seed"}, m.calls)
		require.Equal(t, ext["__item_ids"], m.raw)
	}
}
func TestHookPlanSeedApplicationCapturesBeforePublish(t *testing.T) {
	for _, capable := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "capability"}[capable], func(t *testing.T) {
			ids := idmocks.NewMockIIDGenerator(gomock.NewController(t))
			ids.EXPECT().GenID(gomock.Any()).Return(int64(30), nil)
			m := &seedAppManager{}
			var manager service.IExptManager = m
			if capable {
				manager = seedCapAppManager{m}
			}
			app := &experimentApplication{manager: manager, idgen: ids}
			_, err := app.RunExperiment(context.Background(), &exptpb.RunExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20)), TrialRunItemCount: gptr.Of(int64(99)), Ext: map[string]string{"__item_ids": "[71,72]"}})
			require.Error(t, err)
			require.Equal(t, entity.EvaluationModeTrialRun, m.mode)
			if capable {
				require.Equal(t, []string{"seed", "publish"}, m.calls)
				require.Equal(t, "[71,72]", m.raw)
			} else {
				require.Equal(t, []string{"legacy", "publish"}, m.calls)
			}
		})
	}
}
