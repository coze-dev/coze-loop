// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
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
	"github.com/coze-dev/coze-loop/backend/modules/observability/lib/otel"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestIngestTracesExplicitWorkspace(t *testing.T) {
	ctrl := gomock.NewController(t)
	header := "200"
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	auth.EXPECT().GetClaim(gomock.Any()).Return(nil)
	checked := make(map[string]bool)
	auth.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, id string) error {
		checked[id] = true
		return nil
	}).Times(2)
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
		require.Len(t, checked, 2, "all destinations must be authorized before the first send")
		for _, sp := range req.Spans {
			destinations[sp.SpanID] = sp.WorkspaceID
		}
		return nil
	}).Times(2)
	provider := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
	provider.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), header, gomock.Any()).DoAndReturn(func(_ context.Context, inputs []*span.InputSpan, gotHeader string, _ *rpc.Claim) (string, error) {
		require.Len(t, inputs, 1)
		input := inputs[0]
		require.Equal(t, header, gotHeader)
		if id := input.TagsString["target"]; id != "" {
			return id, nil
		}
		return gotHeader, nil
	}).Times(2)
	application := &OpenAPIApplication{
		auth: auth, workspace: provider, benefit: benefits, tenant: tenant, traceConfig: config,
		traceService: traces, spanContextExtractor: newSpanContextExtractorMock(ctrl),
	}
	_, err := application.IngestTraces(context.Background(), &openapi.IngestTracesRequest{
		WorkspaceID: &header,
		Spans: []*span.InputSpan{
			{WorkspaceID: "100", SpanID: "header"},
			{WorkspaceID: "100", SpanID: "tag", TagsString: map[string]string{"target": "300"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"header": "200", "tag": "300"}, destinations)
}

func TestIngestTracesWorkspacePreflight(t *testing.T) {
	for _, mode := range []string{"resolution error", "invalid resolved target", "permission denied"} {
		t.Run(mode, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			auth.EXPECT().GetClaim(gomock.Any()).Return(nil)
			auth.EXPECT().CheckIngestPermission(gomock.Any(), "200").Return(nil).AnyTimes()
			if mode == "permission denied" {
				auth.EXPECT().CheckIngestPermission(gomock.Any(), "300").Return(errors.New("denied"))
			}
			traces := servicemocks.NewMockITraceService(ctrl)
			traces.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Times(0)
			provider := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
			provider.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), "", gomock.Any()).DoAndReturn(func(_ context.Context, inputs []*span.InputSpan, _ string, _ *rpc.Claim) (string, error) {
				require.Len(t, inputs, 1)
				input := inputs[0]
				if input.SpanID == "valid" {
					return "200", nil
				}
				switch mode {
				case "resolution error":
					return "", errors.New("missing target")
				case "invalid resolved target":
					return "bad", nil
				default:
					return "300", nil
				}
			}).Times(2)
			application := &OpenAPIApplication{auth: auth, workspace: provider, traceService: traces}
			resp, err := application.IngestTraces(context.Background(), &openapi.IngestTracesRequest{
				Spans: []*span.InputSpan{{WorkspaceID: "100", SpanID: "valid"}, {WorkspaceID: "100", SpanID: "invalid"}},
			})
			require.Error(t, err)
			require.Nil(t, resp)
		})
	}
}

func TestIngestTracesEmptyWorkspaceIsSkipped(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	auth.EXPECT().GetClaim(gomock.Any()).Return(nil)
	provider := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
	header := "200"
	provider.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), header, gomock.Any()).Return("", nil)
	application := &OpenAPIApplication{auth: auth, workspace: provider}
	resp, err := application.IngestTraces(context.Background(), &openapi.IngestTracesRequest{
		WorkspaceID: &header, Spans: []*span.InputSpan{{WorkspaceID: "100"}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestIngestTracesNilSpan(t *testing.T) {
	application := &OpenAPIApplication{}
	_, err := application.IngestTraces(context.Background(), &openapi.IngestTracesRequest{Spans: []*span.InputSpan{nil}})
	require.Error(t, err)
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

func TestOpenAPIApplication_OtelIngestTracesWorkspaceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	claim := &rpc.Claim{AuthType: "third_party"}
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	auth.EXPECT().GetClaim(gomock.Any()).Return(claim)
	auth.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Times(0)
	provider := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
	resolveErr := errors.New("missing target workspace")
	provider.EXPECT().GetIngestWorkSpaceID(gomock.Any(), []*span.InputSpan{{
		TagsString: map[string]string{"application_id": "app-1"},
	}}, "", claim).Return("", resolveErr)
	traces := servicemocks.NewMockITraceService(ctrl)
	traces.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Times(0)
	application := &OpenAPIApplication{auth: auth, workspace: provider, traceService: traces}
	resp, err := application.OtelIngestTraces(context.Background(), &openapi.OtelIngestTracesRequest{
		ContentType: otel.ContentTypeJson,
		Body: []byte(`{"resourceSpans":[{"scopeSpans":[{"spans":[
			{"attributes":[{"key":"cozeloop.workspace_id","value":{"stringValue":"200"}}]},
			{"attributes":[{"key":"application_id","value":{"stringValue":"app-1"}}]}
		]}]}]}`),
	})
	require.ErrorIs(t, err, resolveErr)
	require.Nil(t, resp)
}
