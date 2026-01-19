// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracer

import "github.com/coze-dev/coze-loop/backend/pkg/errorx"

// SimpleStatusError 是用于 Trace 上报的瘦身错误类型，仅包含错误码和错误信息，不携带堆栈
type SimpleStatusError struct {
	code int32
	msg  string
}

func (e *SimpleStatusError) Error() string {
	return e.msg
}

func (e *SimpleStatusError) Code() int32 {
	return e.code
}

// SanitizeErrorForTrace 在上报 trace 前，对错误进行瘦身处理：
// - 如果是带 Code 的 StatusError，仅保留 Code 和 Error() 文本
// - 对于普通 error，仅保留 Error() 文本
func SanitizeErrorForTrace(err error) error {
	if err == nil {
		return nil
	}

	if statusErr, ok := errorx.FromStatusError(err); ok {
		return &SimpleStatusError{
			code: statusErr.Code(),
			msg:  statusErr.Error(),
		}
	}

	// 对于没有 Code 的 error，只保留 message
	return &SimpleStatusError{
		code: 0,
		msg:  err.Error(),
	}
}
