package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	domaintarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	domainexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	apitarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	apiexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestConvergedResultSetThriftRejectsExplicitMode(t *testing.T) {
	require.False(t, isValidExptRunModeDTO(&domainexpt.RunModeConfig{RunMode: gptr.Of(domainexpt.ExptRunMode(5))}))
	for _, mode := range []domainexpt.ExptRunMode{domainexpt.ExptRunMode_SingleTurn, domainexpt.ExptRunMode_FixedScriptMultiTurn, domainexpt.ExptRunMode_SuaMultiTurn, domainexpt.ExptRunMode_Goal} {
		require.True(t, isValidExptRunModeDTO(&domainexpt.RunModeConfig{RunMode: &mode}))
	}
}

func TestConvergedResultSetRejectsNumericModeBeforeCreation(t *testing.T) {
	for _, name := range []string{"fresh", "copy"} {
		t.Run(name, func(t *testing.T) {
			manager := servicemocks.NewMockIExptManager(gomock.NewController(t))
			manager.EXPECT().CreateExpt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("unexpected creation boundary reached")).AnyTimes()
			app := &experimentApplication{manager: manager, configer: &appSubmissionConfig{allowMulti: true}}
			req := &apiexpt.CreateExperimentRequest{WorkspaceID: 1001, EvalSetID: gptr.Of(int64(301)), EvalSetVersionID: gptr.Of(int64(401)), RunModeConfig: &domainexpt.RunModeConfig{RunMode: gptr.Of(domainexpt.ExptRunMode(5))}}
			if name == "copy" {
				req.TargetID, req.TargetVersionID = gptr.Of(int64(101)), gptr.Of(int64(102))
			}
			_, err := app.CreateExperiment(context.Background(), req)
			require.ErrorContains(t, err, "invalid run_mode")
		})
	}
}

func TestConvergedResultSetSubmitUsesDomainForRoutingAndMetrics(t *testing.T) {
	for _, mode := range []entity.RunMode{entity.RunModeSingleTurn, entity.RunModeFixedScriptMultiTurn, entity.RunModeSUAMultiTurn, entity.RunModeResultSet} {
		t.Run(mode, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := servicemocks.NewMockIExptManager(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			ids := idgenmocks.NewMockIIDGenerator(ctrl)
			scheduler := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)
			metrics := metricsmocks.NewMockSandboxAgentMetrics(ctrl)
			stored := &entity.Experiment{
				ID: 201, SpaceID: 1001, EvalSetID: 301, EvalSetVersionID: 401,
				TargetID: 101, TargetVersionID: 102,
				Target:   &entity.EvalTarget{EvalTargetType: entity.EvalTargetTypeSandboxAgent, SourceTargetID: "101", EvalTargetVersion: &entity.EvalTargetVersion{SandboxAgent: &entity.SandboxAgent{SandboxCountMode: entity.SandboxCountModeDual}}},
				EvalConf: &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: mode}},
			}
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			manager.EXPECT().CreateExpt(gomock.Any(), gomock.Any(), gomock.Any()).Return(stored, nil)
			ids.EXPECT().GenID(gomock.Any()).Return(int64(501), nil)
			manager.EXPECT().LogRun(gomock.Any(), int64(201), int64(501), gomock.Any(), int64(1001), gomock.Any(), gomock.Any()).Return(nil)
			manager.EXPECT().Run(gomock.Any(), int64(201), int64(501), int64(1001), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			scheduler.EXPECT().Init(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
				require.Equal(t, rpc.SandboxTenantFornaxEvalGeneral, req.Tenant)
				require.Equal(t, int32(10), req.Concurrency)
				return &rpc.SandboxInitResponse{}, nil
			})
			if mode != entity.RunModeResultSet {
				metrics.EXPECT().EmitExperimentStarted(gomock.Any())
			}
			app := &experimentApplication{manager: manager, auth: auth, idgen: ids, sandboxSchedulerAdapter: scheduler, sandboxAgentMetrics: metrics}
			resp, err := app.SubmitExperiment(context.Background(), &apiexpt.SubmitExperimentRequest{
				WorkspaceID: 1001, Name: gptr.Of("app-scoring"), ItemConcurNum: gptr.Of(int32(1)),
				CreateEvalTargetParam: &apitarget.CreateEvalTargetParam{SourceTargetID: gptr.Of("101"), EvalTargetType: gptr.Of(domaintarget.EvalTargetType_SandboxAgent)},
			})
			require.NoError(t, err)
			require.Equal(t, int64(201), resp.GetExperiment().GetID())
			if mode == entity.RunModeResultSet {
				require.Nil(t, resp.GetExperiment().RunModeConfig)
			} else {
				require.NotNil(t, resp.GetExperiment().RunModeConfig)
			}
		})
	}
}

func TestConvergedSubmitPreservesOmittedRunModeTenant(t *testing.T) {
	for _, tc := range []struct {
		name   string
		conf   *domainexpt.RunModeConfig
		tenant rpc.SandboxTenant
	}{
		{"absent", nil, rpc.SandboxTenantFornaxTraeEvalDualSandbox},
		{"empty object", &domainexpt.RunModeConfig{}, rpc.SandboxTenantFornaxEvalGeneral},
		{"budget only", &domainexpt.RunModeConfig{MaxRunMinutes: gptr.Of(int32(30))}, rpc.SandboxTenantFornaxEvalGeneral},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := servicemocks.NewMockIExptManager(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			ids := idgenmocks.NewMockIIDGenerator(ctrl)
			scheduler := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)
			stored := &entity.Experiment{ID: 201, SpaceID: 1001, Target: &entity.EvalTarget{
				EvalTargetType: entity.EvalTargetTypeSandboxAgent,
				EvalTargetVersion: &entity.EvalTargetVersion{SandboxAgent: &entity.SandboxAgent{
					SandboxCountMode: entity.SandboxCountModeDual,
				}},
			}}
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			manager.EXPECT().CreateExpt(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.CreateExptParam, _ *entity.Session) (*entity.Experiment, error) {
				stored.EvalConf = p.ExptConf
				return stored, nil
			})
			scheduler.EXPECT().Init(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *rpc.SandboxInitRequest) (*rpc.SandboxInitResponse, error) {
				require.Equal(t, tc.tenant, p.Tenant)
				require.Equal(t, int32(10), p.Concurrency)
				return &rpc.SandboxInitResponse{}, nil
			})
			ids.EXPECT().GenID(gomock.Any()).Return(int64(501), nil)
			manager.EXPECT().LogRun(gomock.Any(), int64(201), int64(501), gomock.Any(), int64(1001), gomock.Any(), gomock.Any()).Return(nil)
			manager.EXPECT().Run(gomock.Any(), int64(201), int64(501), int64(1001), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			app := &experimentApplication{manager: manager, auth: auth, idgen: ids, sandboxSchedulerAdapter: scheduler}
			_, err := app.SubmitExperiment(context.Background(), &apiexpt.SubmitExperimentRequest{
				WorkspaceID: 1001, Name: gptr.Of("ordinary-dual"), ItemConcurNum: gptr.Of(int32(1)), RunModeConfig: tc.conf,
				CreateEvalTargetParam: &apitarget.CreateEvalTargetParam{SourceTargetID: gptr.Of("102"), EvalTargetType: gptr.Of(domaintarget.EvalTargetType_SandboxAgent)},
			})
			require.NoError(t, err)
			if tc.conf == nil {
				require.Nil(t, stored.EvalConf.RunModeConfig)
			} else {
				require.Nil(t, tc.conf.RunMode)
				require.Empty(t, stored.EvalConf.RunModeConfig.RunMode)
				require.EqualValues(t, tc.conf.GetMaxRunMinutes(), stored.EvalConf.RunModeConfig.MaxRunMinutes)
			}
			require.Equal(t, rpc.SandboxTenantFornaxTraeEvalDualSandbox, sandboxTenantForExperimentEntity(stored))
		})
	}
}
