package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/stretchr/testify/require"
)

func TestFluxWorkflowConsumerSpaceSurvivesExperimentConversion(t *testing.T) {
	request := &openapiExperiment.FluxWorkflowConfig{
		SourceKind: gptr.Of("space-flow"), SourceSpaceID: gptr.Of("origin"),
		ConsumerSpaceID: gptr.Of("consumer"), SourceWorkflowID: gptr.Of("workflow"),
	}
	domain := openAPIFluxWorkflowToDomain(request)
	require.Equal(t, "consumer", domain.GetConsumerSpaceID())
	stored := fluxWorkflowDTO2DO(domain)
	require.Equal(t, "consumer", stored.ConsumerSpaceID)
	back := domainFluxWorkflowToOpenAPI(fluxWorkflowDO2DTO(stored))
	require.Equal(t, "consumer", back.GetConsumerSpaceID())
}

func TestFluxWorkflowOfficialBlackBoxAllowsPhaseReviewWithoutNodes(t *testing.T) {
	config := &openapiExperiment.FluxWorkflowConfig{
		SourceKind: gptr.Of("space-flow"), SourceWorkflowID: gptr.Of("official"),
		PhaseReview: &openapiExperiment.FluxWorkflowPhaseReview{Mode: gptr.Of("auto")},
	}
	require.NoError(t, validateFluxWorkflowOpenAPI(config))
	config.PhaseReview.Mode = gptr.Of("invalid")
	require.Error(t, validateFluxWorkflowOpenAPI(config))
	config.PhaseReview = nil
	require.Error(t, validateFluxWorkflowOpenAPI(config))
	config.SourceKind = gptr.Of("task-template")
	config.SourceWorkflowID = nil
	config.SourceTemplateID = gptr.Of("template")
	config.SourceReleaseID = gptr.Of("release")
	config.PhaseReview = &openapiExperiment.FluxWorkflowPhaseReview{Mode: gptr.Of("manual")}
	require.NoError(t, validateFluxWorkflowOpenAPI(config))
	config.PhaseReview = nil
	require.Error(t, validateFluxWorkflowOpenAPI(config))
}
