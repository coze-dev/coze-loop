// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

const (
	ServiceInternalErrorCode int32 = 1
	DefaultErrorMsg                = "Service Internal Error"
)

func KiteXSvrCompatMW() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) (err error) {
			err = next(ctx, req, resp)
			if err == nil {
				return nil
			}

			if bizErr, ok := kerrors.FromBizStatusError(err); ok {
				ri := rpcinfo.GetRPCInfo(ctx)
				if setter, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok {
					setter.SetBizStatusErr(bizErr)
					return nil
				}
			}

			return err
		}
	}
}

func KiteXSvrErrorWrapMW(opts ...KiteXSvrErrorWrapOptionFn) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) (err error) {
			o := &kiteXSvrErrorWrapOption{}
			for _, opt := range opts {
				opt(o)
			}

			err = next(ctx, req, resp)
			if err == nil {
				return nil
			}

			if kerrors.IsKitexError(err) && !errors.Is(err, kerrors.ErrBiz) {
				return err
			}

			be := getBizStatusError(err, o.getWrapErrorCode())

			ri := rpcinfo.GetRPCInfo(ctx)
			if setter, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok {
				setter.SetBizStatusErr(be)
				return nil
			}

			return be
		}
	}
}

func getBizStatusError(err error, wrapCode int32) kerrors.BizStatusErrorIface {
	var detailedErr *kerrors.DetailedError
	if errors.As(err, &detailedErr) {
		unknownErr := detailedErr.Unwrap()
		return kerrors.NewBizStatusError(wrapCode, fmt.Sprintf("%s:%s", DefaultErrorMsg, errorx.ErrorWithoutStack(unknownErr)))
	}
	statusErr, ok := kerrors.FromBizStatusError(errorx.NewByCode(wrapCode))
	if ok {
		return statusErr
	}
	return kerrors.NewBizStatusError(wrapCode, fmt.Sprintf("%s:%v", DefaultErrorMsg, errorx.ErrorWithoutStack(err)))
}

type kiteXSvrErrorWrapOption struct {
	WrapErrorCode int32
}

func (k *kiteXSvrErrorWrapOption) getWrapErrorCode() int32 {
	if k.WrapErrorCode > 0 {
		return k.WrapErrorCode
	}
	return ServiceInternalErrorCode
}

type KiteXSvrErrorWrapOptionFn func(opt *kiteXSvrErrorWrapOption)

func WithWrapErrorCode(code int32) KiteXSvrErrorWrapOptionFn {
	return func(c *kiteXSvrErrorWrapOption) {
		c.WrapErrorCode = code
	}
}
