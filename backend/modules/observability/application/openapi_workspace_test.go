// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/openapi"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	workspacemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/workspace/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestIngestTracesWorkspaceHeaderForwarding(t *testing.T) {
	ctrl := gomock.NewController(t)
	header := "200"
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	claim := &rpc.Claim{AuthType: "test"}
	auth.EXPECT().GetClaim(gomock.Any()).Return(claim)
	auth.EXPECT().CheckIngestPermission(gomock.Any(), "200").Return(nil)
	auth.EXPECT().CheckIngestPermission(gomock.Any(), "300").Return(nil)
	benefits := benefitmocks.NewMockIBenefitService(ctrl)
	benefits.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
		AccountAvailable: true, IsEnough: true, StorageDuration: 3,
	}, nil).Times(2)
	tenant := tenantmocks.NewMockITenantProvider(ctrl)
	tenant.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("t").Times(2)
	config := configmocks.NewMockITraceConfig(ctrl)
	config.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(nil, nil).Times(2)
	traces := servicemocks.NewMockITraceService(ctrl)
	destinations := make(map[string]string)
	traces.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *service.IngestTracesReq) error {
		for _, sp := range req.Spans {
			destinations[sp.SpanID] = sp.WorkspaceID
		}
		return nil
	}).Times(2)
	provider := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
	inputSpans := []*span.InputSpan{
		{WorkspaceID: "100", SpanID: "header"},
		{WorkspaceID: "100", SpanID: "tag", TagsString: map[string]string{"target": "300"}},
	}
	provider.EXPECT().GetIngestWorkSpaceID(gomock.Any(), []*span.InputSpan{inputSpans[0]}, header, claim).Return("200")
	provider.EXPECT().GetIngestWorkSpaceID(gomock.Any(), []*span.InputSpan{inputSpans[1]}, header, claim).Return("300")

	application := &OpenAPIApplication{
		auth: auth, workspace: provider, benefit: benefits, tenant: tenant, traceConfig: config,
		traceService: traces, spanContextExtractor: newSpanContextExtractorMock(ctrl),
	}
	_, err := application.IngestTraces(context.Background(), &openapi.IngestTracesRequest{
		WorkspaceID: &header,
		Spans:       inputSpans,
	})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"header": "200", "tag": "300"}, destinations)
}

func TestIngestTracesWorkspaceHeaderBinding(t *testing.T) {
	c := app.NewContext(0)
	c.Request.Header.SetMethod("POST")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("cozeloop-workspace-id", "200")
	c.Request.SetBodyString(`{"spans":[{"workspace_id":"100"}]}`)
	c.Request.Header.SetContentLength(len(c.Request.Body()))
	var req openapi.IngestTracesRequest
	require.NoError(t, c.BindAndValidate(&req))
	require.Equal(t, "200", req.GetWorkspaceID())
	require.Len(t, req.Spans, 1)
	require.Equal(t, "100", req.Spans[0].WorkspaceID)
}
