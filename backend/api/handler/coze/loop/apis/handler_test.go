// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type helloReq struct {
	Name string
}

type helloResp struct {
	Message string
}

type helloService interface {
	Hello(ctx context.Context, req *helloReq) (*helloResp, error)
}

type helloClient interface {
	Hello(ctx context.Context, req *helloReq, callOptions ...callopt.Option) (*helloResp, error)
}

type mockHelloService struct {
	mock.Mock
}

func (m *mockHelloService) Hello(ctx context.Context, req *helloReq) (*helloResp, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*helloResp), args.Error(1)
}

type mockHelloClient struct {
	helloService
}

func (m *mockHelloClient) Hello(ctx context.Context, req *helloReq, callOptions ...callopt.Option) (*helloResp, error) {
	return m.helloService.Hello(ctx, req)
}

func newHelloClient(hs helloService, mws ...endpoint.Middleware) *mockHelloClient {
	return &mockHelloClient{helloService: hs}
}

func Test_bindLocalCallClient(t *testing.T) {
	tests := []struct {
		name     string
		svc      helloService
		cli      any
		provider func(helloService, ...endpoint.Middleware) *mockHelloClient
	}{
		{
			name:     "successful binding",
			svc:      &mockHelloService{},
			cli:      ptr.Of(func() helloClient { return &mockHelloClient{} }()),
			provider: newHelloClient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				bindLocalCallClient(tt.svc, tt.cli, tt.provider)
			})
		})
	}
}

func Test_invokeAndRender(t *testing.T) {
	tests := []struct {
		name     string
		req      interface{}
		callable func(ctx context.Context, req *helloReq, callOptions ...callopt.Option) (*helloResp, error)
		wantErr  bool
		errCode  int
	}{
		{
			name: "successful invocation",
			req:  &helloReq{Name: "test"},
			callable: func(ctx context.Context, req *helloReq, callOptions ...callopt.Option) (*helloResp, error) {
				return &helloResp{Message: "Hello test"}, nil
			},
			wantErr: false,
		},
		{
			name: "service error",
			req:  &helloReq{Name: "test"},
			callable: func(ctx context.Context, req *helloReq, callOptions ...callopt.Option) (*helloResp, error) {
				return nil, errors.New("service error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			c := &app.RequestContext{}

			invokeAndRender(ctx, c, tt.callable)

			if tt.wantErr {
				assert.Error(t, c.Errors.Last())
			} else {
				assert.Equal(t, http.StatusOK, c.Response.StatusCode())
			}
		})
	}
}
