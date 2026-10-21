package experiment

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
	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	apiExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	service "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type submissionContractGate struct {
	component.IConfiger
	defaultVersion string
	requested      []string
}

func (g *submissionContractGate) ResolveResultSetRuntime(_ context.Context, _ int64, requested string) (string, error) {
	g.requested = append(g.requested, requested)
	if requested == "" {
		return g.defaultVersion, nil
	}
	return requested, nil
}
func (g *submissionContractGate) CheckResultSetRuntime(_ context.Context, _ int64, version string) error {
	if version != "result-set-test" {
		return fmt.Errorf("unsupported result_set version")
	}
	return nil
}

// These are exact frontend JSON.stringify outputs. Only the IDL api.js_conv
// int64 adaptation performed by AGW is reproduced before the real DTO converter.
func loadSerializedResultSetRequest(t *testing.T, name string) *apiExpt.CreateExperimentRequest {
	t.Helper()
	raw, err := os.ReadFile("testdata/result_set_submit/" + name + ".json")
	require.NoError(t, err)
	var manifest struct {
		FrontendCommit string            `json:"frontend_commit"`
		Files          map[string]string `json:"files"`
	}
	manifestRaw, err := os.ReadFile("testdata/result_set_submit/manifest.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(manifestRaw, &manifest))
	require.Equal(t, manifest.Files[name], fmt.Sprintf("%x", sha256.Sum256(raw)))
	t.Logf("frontend payload %s sha256=%x", name, sha256.Sum256(raw))
	var value any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	require.NoError(t, dec.Decode(&value))
	var convertIDs func(any)
	convertIDs = func(v any) {
		switch typed := v.(type) {
		case map[string]any:
			for key, child := range typed {
				switch key {
				case "workspace_id", "eval_set_id", "eval_set_version_id", "source_space_id":
					if text, ok := child.(string); ok {
						_, err := strconv.ParseInt(text, 10, 64)
						require.NoError(t, err)
						typed[key] = json.Number(text)
					}
				}
				convertIDs(child)
			}
		case []any:
			for _, child := range typed {
				convertIDs(child)
			}
		}
	}
	convertIDs(value)
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	var req apiExpt.CreateExperimentRequest
	require.NoError(t, json.Unmarshal(encoded, &req))
	return &req
}

func TestFrontendSerializedResultSetReachesCreateExptPreflight(t *testing.T) {
	for _, name := range []string{"fresh", "copy", "ab"} {
		t.Run(name, func(t *testing.T) {
			req := loadSerializedResultSetRequest(t, name)
			if name == "fresh" {
				require.Nil(t, req.TargetRuntimeParam)
			} else {
				require.JSONEq(t, `{"binary_version":"result-set-test"}`, req.TargetRuntimeParam.GetJSONValue())
			}
			param, err := ConvertCreateReq(req, nil)
			require.NoError(t, err)
			require.Equal(t, entity.ExptEvalSetSourceType_MultiSetConfig, param.EvalSetSourceType)
			wantVersion := int64(2001)
			if name == "ab" {
				wantVersion = 2002
			}
			require.Len(t, param.EvalSetConfigs, 1)
			require.Equal(t, wantVersion, param.EvalSetConfigs[0].EvalSetVersionID)
			gate := &submissionContractGate{defaultVersion: "result-set-test"}
			wantRequested := ""
			if name != "fresh" {
				gate.defaultVersion = "different-system-default"
				wantRequested = "result-set-test"
			}
			boundary := errors.New("contract reached system target creation")
			target := svcmocks.NewMockIEvalTargetService(gomock.NewController(t))
			target.EXPECT().CreateEvalTarget(gomock.Any(), int64(1000), entity.BuiltinResultSetTargetID, gomock.Any(), entity.EvalTargetTypeSandboxAgent, gomock.Any()).Return(int64(0), int64(0), boundary)
			manager := newSubmissionContractManager(gate, target)
			_, err = manager.CreateExpt(context.Background(), param, nil)
			require.ErrorIs(t, err, boundary)
			require.Equal(t, []string{wantRequested}, gate.requested)
			versions, err := param.ExptConf.ResultSetBinaryVersions()
			require.NoError(t, err)
			require.Len(t, versions, 2)
			for _, version := range versions {
				require.Equal(t, "result-set-test", version)
			}
			require.NotNil(t, param.ExptConf.ConnectorConf.TargetConf.IngressConf.EvalSetAdapter)
		})
	}
}

func TestResultSetSingleSelectionNormalizesBeforeCreate(t *testing.T) {
	for _, sourceType := range []*domainExpt.ExptEvalSetSourceType{nil, gptr.Of(domainExpt.ExptEvalSetSourceType_SingleSet)} {
		req := loadSerializedResultSetRequest(t, "fresh")
		req.EvalSetSourceType = sourceType
		req.EvalSetConfigs = nil
		param, err := ConvertCreateReq(req, nil)
		require.NoError(t, err)
		require.Equal(t, entity.ExptEvalSetSourceType_MultiSetConfig, param.EvalSetSourceType)
		require.Len(t, param.EvalSetConfigs, 1)
		require.EqualValues(t, 2001, param.EvalSetConfigs[0].EvalSetVersionID)
	}
	req := loadSerializedResultSetRequest(t, "fresh")
	req.EvalSetSourceType = gptr.Of(domainExpt.ExptEvalSetSourceType_SingleSet)
	_, err := ConvertCreateReq(req, nil)
	require.ErrorContains(t, err, "conflicts")
	req = loadSerializedResultSetRequest(t, "ab")
	req.EvalSetVersionID = gptr.Of(int64(2001))
	_, err = ConvertCreateReq(req, nil)
	require.ErrorContains(t, err, "conflicts")
	req = loadSerializedResultSetRequest(t, "fresh")
	req.EvalSetConfigs = nil
	req.EvalSetSourceType = nil
	req.EvalSetVersionID = nil
	_, err = ConvertCreateReq(req, nil)
	require.ErrorContains(t, err, "fixed evaluation set")
	ordinary := &apiExpt.CreateExperimentRequest{EvalSetID: gptr.Of(int64(1)), EvalSetVersionID: gptr.Of(int64(2))}
	require.NoError(t, NormalizeResultSetCreateRequest(ordinary))
	require.Nil(t, ordinary.EvalSetSourceType)
	require.Nil(t, ordinary.EvalSetConfigs)
}

func newSubmissionContractManager(gate component.IConfiger, target service.IEvalTargetService) service.IExptManager {
	return service.NewExptManager(
		nil,    // exptResultService
		nil,    // exptRepo
		nil,    // exptRunLogRepo
		nil,    // exptStatsRepo
		nil,    // exptItemResultRepo
		nil,    // exptItemRefRepo
		nil,    // exptTurnResultRepo
		gate,   // configer
		nil,    // quotaRepo
		nil,    // mutex
		nil,    // idem
		nil,    // publisher
		nil,    // audit
		nil,    // idgen
		nil,    // metric
		nil,    // lwt
		nil,    // evaluationSetVersionService
		nil,    // evaluationSetService
		target, // evalTargetService
		nil,    // evaluatorService
		nil,    // benefitService
		nil,    // exptAggrResultService
		nil,    // templateRepo
		nil,    // templateManager
		nil,    // notifyRPCAdapter
		nil,    // userProvider
		nil,    // pipelineListAdapter
		nil,    // resourceAccessAuthorizer
		nil,    // sandboxAgentMetrics
		nil,    // centralScopeProvider
		nil,    // centralAdmissionPolicy
		nil,    // centralGuard
	)
}
