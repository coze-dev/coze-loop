package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domainexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	apiexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type appSubmissionConfig struct {
	component.IConfiger
	applicationID string
	allowMulti    bool
}

func (g *appSubmissionConfig) IsResultSetApplication(_ context.Context, id string) bool {
	return id == g.applicationID
}
func (g *appSubmissionConfig) ResolveResultSetRuntime(_ context.Context, _ int64, requested string) (string, error) {
	if requested == "" {
		return "result-set-test", nil
	}
	return requested, nil
}
func (g *appSubmissionConfig) CheckResultSetRuntime(_ context.Context, _ int64, version string) error {
	if version != "result-set-test" {
		return fmt.Errorf("unsupported result-set runtime")
	}
	return nil
}
func (g *appSubmissionConfig) GetExptMultiSetWhiteList(context.Context) *entity.ExptMultiSetWhiteList {
	return &entity.ExptMultiSetWhiteList{AllowAll: g.allowMulti}
}

type observedAppSubmissionManager struct {
	service.IExptManager
	param *entity.CreateExptParam
}

func (m *observedAppSubmissionManager) CreateExpt(ctx context.Context, p *entity.CreateExptParam, session *entity.Session) (*entity.Experiment, error) {
	m.param = p
	return m.IExptManager.CreateExpt(ctx, p, session)
}

// The fixture is the real frontend getSubmitValues JSON output; AGW adapts only these int64 fields.
func serializedApplicationSubmission(t *testing.T) *apiexpt.SubmitExperimentRequest {
	t.Helper()
	raw, err := os.ReadFile("testdata/result_set_application/legacy-app-serialized-payload.json")
	require.NoError(t, err)
	t.Logf("frontend serializer fixture sha256=%x", sha256.Sum256(raw))
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&value))
	var adaptIDs func(any)
	adaptIDs = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				switch key {
				case "workspace_id", "eval_set_id", "eval_set_version_id", "source_space_id":
					if text, ok := child.(string); ok {
						_, err := strconv.ParseInt(text, 10, 64)
						require.NoError(t, err)
						v[key] = json.Number(text)
					}
				}
				adaptIDs(child)
			}
		case []any:
			for _, child := range v {
				adaptIDs(child)
			}
		}
	}
	adaptIDs(value)
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	var req apiexpt.SubmitExperimentRequest
	require.NoError(t, json.Unmarshal(encoded, &req))
	require.Nil(t, req.RunModeConfig)
	require.Nil(t, req.EvalSetSourceType)
	require.Empty(t, req.EvalSetConfigs)
	require.Nil(t, req.TargetRuntimeParam)
	return &req
}

func TestSerializedLegacyApplicationSubmitNormalizesSingleSetAtTrustedBoundary(t *testing.T) {
	for _, name := range []string{"fresh", "explicit single-set", "mapped shared set with pin", "ordinary application", "missing version", "multi-set gate denied"} {
		t.Run(name, func(t *testing.T) {
			req := serializedApplicationSubmission(t)
			gate := &appSubmissionConfig{applicationID: req.CreateEvalTargetParam.GetSourceTargetID(), allowMulti: name != "multi-set gate denied"}
			if name == "explicit single-set" {
				req.EvalSetSourceType = gptr.Of(domainexpt.ExptEvalSetSourceType_SingleSet)
			}
			if name == "ordinary application" {
				req.CreateEvalTargetParam.SourceTargetID = gptr.Of("102")
			}
			if name == "missing version" {
				req.EvalSetVersionID = nil
			}
			if name == "mapped shared set with pin" {
				req.EvalSetSharedOption = &common.SharedResourceOption{IsShared: gptr.Of(true), SourceSpaceID: gptr.Of(int64(2002))}
				req.TargetFieldMapping = &domainexpt.TargetFieldMapping{FromEvalSet: []*domainexpt.FieldMapping{{FieldName: gptr.Of("FORNAX_output"), FromFieldName: gptr.Of("imported_answer")}, {FieldName: gptr.Of("run_conf"), FromFieldName: gptr.Of("original_turns")}}}
				req.TargetRuntimeParam = &common.RuntimeParam{JSONValue: gptr.Of(`{"binary_version":"result-set-test"}`)}
			}
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			var sharedAuth service.ResourceAccessAuthorizer
			if name == "mapped shared set with pin" {
				mock := svcmocks.NewMockResourceAccessAuthorizer(ctrl)
				mock.EXPECT().AuthorizeRead(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, request *entity.AuthorizeResourceRequest) (*entity.ResourceAccessContext, error) {
					require.Equal(t, req.GetEvalSetID(), request.ResourceID)
					require.Equal(t, int64(2002), *request.SharedOption.SourceSpaceID)
					return &entity.ResourceAccessContext{ResourceSpaceID: 2002, AccessMode: entity.AccessModeShared, VersionPolicy: entity.SharedVersionPolicyAll}, nil
				})
				sharedAuth = mock
			}
			boundary := errors.New("reached registered application lookup")
			target := svcmocks.NewMockIEvalTargetService(ctrl)
			if name != "missing version" && name != "multi-set gate denied" {
				target.EXPECT().CreateEvalTarget(gomock.Any(), req.WorkspaceID, req.CreateEvalTargetParam.GetSourceTargetID(), gomock.Any(), entity.EvalTargetTypeSandboxAgent, gomock.Any()).Return(int64(0), int64(0), boundary)
			}
			manager := &observedAppSubmissionManager{IExptManager: service.NewExptManager(
				nil, nil, nil, nil, nil, nil, nil, gate,
				nil, nil, nil, nil, nil, nil, nil, nil,
				nil, nil, target, nil, nil, nil, nil, nil, nil, nil, nil, sharedAuth,
				nil, nil, nil, nil,
			)}
			app := &experimentApplication{manager: manager, configer: gate, auth: auth}
			_, err := app.SubmitExperiment(context.Background(), req)
			if name == "missing version" {
				require.ErrorContains(t, err, "fixed evaluation set")
				return
			}
			if name == "multi-set gate denied" {
				require.ErrorContains(t, err, "multi-set")
				return
			}
			require.ErrorIs(t, err, boundary)
			param := manager.param
			if name == "ordinary application" {
				require.False(t, param.ExptConf.IsResultSet())
				require.Empty(t, param.EvalSetConfigs)
				return
			}
			require.Equal(t, entity.ExptEvalSetSourceType_MultiSetConfig, param.EvalSetSourceType)
			require.Len(t, param.EvalSetConfigs, 1)
			set := param.EvalSetConfigs[0]
			require.Equal(t, req.GetEvalSetID(), set.EvalSetID)
			require.Equal(t, req.GetEvalSetVersionID(), set.EvalSetVersionID)
			require.Equal(t, []string{"result-set-test", "result-set-test"}, func() []string { v, err := param.ExptConf.ResultSetBinaryVersions(); require.NoError(t, err); return v }())
			require.Len(t, set.TargetConfs, 1)
			require.JSONEq(t, `{"binary_version":"result-set-test"}`, set.TargetConfs[0].RuntimeParam[consts.FieldAdapterBuiltinFieldNameRuntimeParam])
			if name == "mapped shared set with pin" {
				require.Equal(t, int64(2002), *set.SharedOption.SourceSpaceID)
				want := []*entity.FieldConf{{FieldName: "FORNAX_output", FromField: "imported_answer"}, {FieldName: "run_conf", FromField: "original_turns"}}
				require.Equal(t, want, set.TargetConfs[0].FieldMapping)
				require.Equal(t, want, param.ExptConf.ConnectorConf.TargetConf.IngressConf.EvalSetAdapter.FieldConfs)
			}
		})
	}
}
