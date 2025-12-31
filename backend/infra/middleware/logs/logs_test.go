// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type mockBizErr struct {
	errorCode int32
	extra     map[string]string
}

func (e *mockBizErr) BizStatusCode() int32        { return e.errorCode }
func (e *mockBizErr) BizMessage() string          { return "biz error" }
func (e *mockBizErr) BizExtra() map[string]string { return e.extra }
func (e *mockBizErr) Error() string               { return "biz error" }

func TestLogTrafficMW(t *testing.T) {
	origLevel := logs.DefaultLogger().GetLevel()
	defer logs.SetLogLevel(origLevel)

	tests := []struct {
		name      string
		logLevel  logs.LogLevel
		rpcInfo   rpcinfo.RPCInfo
		nextErr   error
		expectErr error
	}{
		{
			name:     "no rpc info, success",
			logLevel: logs.InfoLevel,
			rpcInfo:  nil,
			nextErr:  nil,
		},
		{
			name:     "with rpc info, success",
			logLevel: logs.InfoLevel,
			rpcInfo: func() rpcinfo.RPCInfo {
				ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", "MethodA"), nil, nil)
				return ri
			}(),
			nextErr: nil,
		},
		{
			name:     "with rpc info, success, logging disabled",
			logLevel: logs.WarnLevel,
			rpcInfo: func() rpcinfo.RPCInfo {
				ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", "MethodA"), nil, nil)
				return ri
			}(),
			nextErr: nil,
		},
		{
			name:     "rpc info without To, success",
			logLevel: logs.InfoLevel,
			rpcInfo: func() rpcinfo.RPCInfo {
				ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", "MethodA"), nil, nil)
				// rpcinfo.NewRPCInfo's second arg is 'to'. If it's nil, To() returns nil.
				return ri
			}(),
			nextErr: nil,
		},
		{
			name:     "normal error",
			logLevel: logs.InfoLevel,
			rpcInfo: func() rpcinfo.RPCInfo {
				return rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("srv", "MethodB", nil, nil), rpcinfo.NewInvocation("", "MethodB"), nil, nil)
			}(),
			nextErr:   errors.New("some error"),
			expectErr: errors.New("some error"),
		},
		{
			name:     "biz error in next return",
			logLevel: logs.InfoLevel,
			rpcInfo: func() rpcinfo.RPCInfo {
				return rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("srv", "MethodC", nil, nil), rpcinfo.NewInvocation("", "MethodC"), nil, nil)
			}(),
			nextErr: &mockBizErr{
				errorCode: 100,
				extra:     map[string]string{extraKeyAffectStability: "1"},
			},
			expectErr: &mockBizErr{
				errorCode: 100,
				extra:     map[string]string{extraKeyAffectStability: "1"},
			},
		},
		{
			name:     "biz error in rpcinfo",
			logLevel: logs.InfoLevel,
			rpcInfo: func() rpcinfo.RPCInfo {
				bizErr := kerrors.NewBizStatusError(200, "biz err in info")
				ri := rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("srv", "MethodD", nil, nil), rpcinfo.NewInvocation("", "MethodD"), nil, nil)
				ri.Invocation().(interface {
					SetBizStatusErr(err kerrors.BizStatusErrorIface)
				}).SetBizStatusErr(bizErr)
				return ri
			}(),
			nextErr: nil,
		},
		{
			name:     "biz error without stability affect",
			logLevel: logs.InfoLevel,
			rpcInfo: func() rpcinfo.RPCInfo {
				return rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("srv", "MethodE", nil, nil), rpcinfo.NewInvocation("", "MethodE"), nil, nil)
			}(),
			nextErr: &mockBizErr{
				errorCode: 300,
				extra:     map[string]string{extraKeyAffectStability: "0"},
			},
			expectErr: &mockBizErr{
				errorCode: 300,
				extra:     map[string]string{extraKeyAffectStability: "0"},
			},
		},
		{
			name:     "simplify methods - IngestTraces",
			logLevel: logs.InfoLevel,
			rpcInfo: func() rpcinfo.RPCInfo {
				return rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("srv", "IngestTraces", nil, nil), rpcinfo.NewInvocation("", "IngestTraces"), nil, nil)
			}(),
			nextErr: &mockBizErr{
				errorCode: 400,
			},
			expectErr: &mockBizErr{
				errorCode: 400,
			},
		},
		{
			name:     "debug success",
			logLevel: logs.DebugLevel,
			rpcInfo: func() rpcinfo.RPCInfo {
				return rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("srv", "MethodF", nil, nil), rpcinfo.NewInvocation("", "MethodF"), nil, nil)
			}(),
			nextErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs.SetLogLevel(tt.logLevel)
			ctx := context.Background()
			if tt.rpcInfo != nil {
				ctx = rpcinfo.NewCtxWithRPCInfo(ctx, tt.rpcInfo)
			}

			mw := LogTrafficMW(func(ctx context.Context, req, resp any) error {
				return tt.nextErr
			})

			req := "request"
			resp := "response"
			err := mw(ctx, req, resp)

			if tt.expectErr != nil {
				assert.Equal(t, tt.expectErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
