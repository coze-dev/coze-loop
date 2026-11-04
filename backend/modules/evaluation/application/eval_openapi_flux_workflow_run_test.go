package application

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/stretchr/testify/require"
)

func TestOSSFluxWorkflowCopyYAMLIsFailClosed(t *testing.T) {
	resp, err := (&EvalOpenAPIApplication{}).GetFluxWorkflowCopyYAML(context.Background(), &openapi.GetFluxWorkflowCopyYAMLRequest{})
	require.Nil(t, resp)
	require.Error(t, err)
}
