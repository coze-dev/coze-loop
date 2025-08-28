// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"errors"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"

	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	extraKeyAffectStability = "biz_err_affect_stability"
)

func LogTrafficMW(next endpoint.Endpoint) endpoint.Endpoint {
	disabled := func() bool {
		return logs.DefaultLogger().GetLevel() > logs.InfoLevel
	}

	return func(ctx context.Context, req, resp any) (err error) {
		var (
			info   = rpcinfo.GetRPCInfo(ctx)
			bizErr kerrors.BizStatusErrorIface
			to     = "unknown"
		)
		if info != nil && info.To() != nil {
			to = info.To().Method()
		}

		logs.CtxInfo(ctx, "[%s] start", to)
		defer func() {
			logs.CtxInfo(ctx, "[%s] end", to)
		}()
		err = next(ctx, req, resp)
		if err == nil && disabled() {
			return err
		}

		if info != nil && info.Invocation() != nil && info.Invocation().BizStatusErr() != nil {
			bizErr = info.Invocation().BizStatusErr()
		}
		if bizErr == nil && err != nil {
			errors.As(err, &bizErr)
		}

		switch {
		case err != nil && bizErr == nil:
			logs.CtxError(ctx, "RPC %s failed, req=%s, err=%v", to, json.Jsonify(req), err)

		case bizErr != nil:
			if v := bizErr.BizExtra()[extraKeyAffectStability]; v == "1" {
				logs.CtxError(ctx, "RPC %s failed, req=%s, biz_err=%+v, resp=%s", to, json.Jsonify(req), bizErr, json.Jsonify(resp))
			} else {
				logs.CtxWarn(ctx, "RPC %s failed, req=%s, biz_err=%+v, resp=%s", to, json.Jsonify(req), bizErr, json.Jsonify(resp))
			}

		default:
			logs.CtxDebug(ctx, "RPC %s succeeded, req=%s, resp=%s", to, json.Jsonify(req), json.Jsonify(resp))
		}

		return err
	}
}
