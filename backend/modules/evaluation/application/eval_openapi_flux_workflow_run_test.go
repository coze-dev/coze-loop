package application

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

type fluxRunAuthAsyncRepo struct {
	repo.IEvalAsyncRepo
	ctx *entity.EvalAsyncCtx
}

func (r fluxRunAuthAsyncRepo) GetEvalAsyncCtx(context.Context, string) (*entity.EvalAsyncCtx, error) {
	return r.ctx, nil
}

type fluxRunAuthBindingRepo struct {
	repo.IFluxWorkflowBindingRepo
}

func (fluxRunAuthBindingRepo) GetFluxWorkflowBinding(_ context.Context, spaceID, exptID int64) (*entity.FluxWorkflowBinding, error) {
	if spaceID != 12 || exptID != 22 {
		return nil, fmt.Errorf("wrong experiment owner: space=%d experiment=%d", spaceID, exptID)
	}
	return &entity.FluxWorkflowBinding{BindingID: "binding", State: "ready"}, nil
}

func TestFluxRunAuthorizationUsesExperimentSpaceForSharedTarget(t *testing.T) {
	app := &EvalOpenAPIApplication{
		asyncRepo: fluxRunAuthAsyncRepo{ctx: &entity.EvalAsyncCtx{
			RecordID: 33, Callee: sandboxAgentAsyncCallee,
			Event: &entity.ExptItemEvalEvent{SpaceID: 12, ExptID: 22},
		}},
		fluxBindingRepo: fluxRunAuthBindingRepo{},
	}
	exptID, exptSpaceID, bindingID, err := app.authorizeFluxWorkflowRun(context.Background(), 11, 33, nil)
	require.NoError(t, err)
	require.Equal(t, int64(22), exptID)
	require.Equal(t, int64(12), exptSpaceID)
	require.Equal(t, "binding", bindingID)
	var next entity.FluxItemRun
	require.NoError(t, json.Unmarshal([]byte(`{"experiment_id":22,"space_id":11,"invoke_id":"33","binding_id":"binding","state":"reserved"}`), &next))
	next.ExperimentSpaceID = exptSpaceID
	encoded, err := json.Marshal(&next)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"experiment_space_id":12`)

	app.asyncRepo = fluxRunAuthAsyncRepo{}
	_, _, _, err = app.authorizeFluxWorkflowRun(context.Background(), 11, 33, &entity.FluxItemRun{
		SpaceID: 11, InvokeID: 33, ExperimentID: 22, ExperimentSpaceID: 12, BindingID: "binding",
	})
	require.NoError(t, err)
}

func TestOSSFluxWorkflowCopyYAMLIsFailClosed(t *testing.T) {
	resp, err := (&EvalOpenAPIApplication{}).GetFluxWorkflowCopyYAML(context.Background(), &openapi.GetFluxWorkflowCopyYAMLRequest{})
	require.Nil(t, resp)
	require.Error(t, err)
}
