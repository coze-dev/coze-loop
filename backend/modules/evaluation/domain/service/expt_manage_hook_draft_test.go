// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestHookManagerDraftSetVersionReferences(t *testing.T) {
	for _, tc := range []struct {
		name      string
		multi     bool
		versionID int64
	}{
		{name: "single_set_draft", versionID: 71},
		{name: "multi_set_draft", multi: true, versionID: 71},
		{name: "physical_version", versionID: 72},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager := newTestExptManager(gomock.NewController(t))
			versions := manager.evaluationSetVersionService.(*svcmocks.MockEvaluationSetVersionService)
			expt := &entity.Experiment{
				ID: 20, SpaceID: 10, Name: "set reference", ExptType: entity.ExptType_Online,
				EvalSetID: 71, EvalSetVersionID: tc.versionID,
			}
			if tc.multi {
				expt.EvalConf = &entity.EvaluationConfiguration{
					EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 71, EvalSetVersionID: 71}},
				}
			}
			mode := entity.EvaluationModeAppend
			if tc.versionID == 72 {
				expt.ExptType, mode = entity.ExptType_Offline, entity.EvaluationModeSubmit
				versions.EXPECT().GetEvaluationSetVersion(gomock.Any(), int64(10), int64(72), gptr.Of(true), nil).
					Return(&entity.EvaluationSetVersion{ID: 72, SpaceID: 10, EvaluationSetID: 71, Version: "v1"}, nil, nil)
			} else {
				// Allow the old lookup so RED is a behavior failure, not an unexpected mock call.
				versions.EXPECT().GetEvaluationSetVersion(gomock.Any(), int64(10), int64(71), gptr.Of(true), nil).
					Return(nil, nil, errors.New("physical version not found: draft uses dataset ID")).AnyTimes()
			}

			got, err := manager.hookRunContext(context.Background(), expt,
				&entity.ExptRunLog{SpaceID: 10, ExptID: 20, ExptRunID: 30, Mode: int32(mode)},
				&spi.HookInitiator{UserID: gptr.Of("123"), IdentityType: gptr.Of("fornax_user")})
			require.NoError(t, err, "draft aliases must remain valid without a physical version")
			require.Len(t, got.EvalSets, 1)
			require.Equal(t, "10", got.EvalSets[0].GetWorkspaceID())
			require.Equal(t, "71", got.EvalSets[0].GetID())
			if tc.versionID == 72 {
				require.Equal(t, "72", got.EvalSets[0].GetVersionID())
				require.Equal(t, "v1", got.EvalSets[0].GetVersion())
			} else {
				require.Nil(t, got.EvalSets[0].VersionID)
				require.Nil(t, got.EvalSets[0].Version)
			}
		})
	}
}
