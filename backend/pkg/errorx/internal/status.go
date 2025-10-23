// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/kitex/pkg/kerrors"
)

// StatusError is interface for error with statusError.
// 如果有获取code或其他扩展字段的需求，再考虑对外暴露接口.
type StatusError interface {
	error
	Code() int32
}

type statusError struct {
	// 核心属性
	statusCode int32  // 错误码, 错误类型的服务间标识
	message    string // 错误码描述信息

	// 错误码的扩展属性
	ext Extension
}

type withStatus struct {
	// DTO, 会进行服务间流转
	status *statusError

	// 下面的属性只在服务内生效
	stack string
	cause error // 原始的错误信息
}

type Extension struct {
	IsAffectStability bool              // 稳定性标识 可用于SLA稳定的监测. true:会影响系统稳定性, 并体现在接口错误率中, false:不影响稳定性
	Extra             map[string]string // 扩展信息
}

func (w *statusError) Code() int32 {
	return w.statusCode
}

func (w *statusError) Error() string {
	return fmt.Sprintf("code=%d message=%s", w.statusCode, w.message)
}

func (w *statusError) IsAffectStability() bool {
	return w.ext.IsAffectStability
}

func (w *statusError) WithAffectStability(affectStability bool) {
	w.ext.IsAffectStability = affectStability
}

func (w *statusError) Extra() map[string]string {
	return w.ext.Extra
}

func (w *statusError) WithExtra(m map[string]string) {
	if w.ext.Extra == nil {
		w.ext.Extra = make(map[string]string)
	}
	for k, v := range m {
		w.ext.Extra[k] = v
	}
}

// Unwrap supports go errors.Unwrap().
func (w *withStatus) Unwrap() error {
	return w.cause
}

// Is supports go errors.Is().
func (w *withStatus) Is(target error) bool {
	var ws StatusError
	if errors.As(target, &ws) && w.status.Code() == ws.Code() {
		return true
	}
	return false
}

// As supports go errors.As().
func (w *withStatus) As(target interface{}) bool {
	return errors.As(w.status, target)
}

func (w *withStatus) StackTrace() string {
	return w.stack
}

func (w *withStatus) Error() string {
	b := strings.Builder{}
	b.WriteString(w.status.Error())

	if w.cause != nil {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("cause=%s", w.cause))
	}

	if w.stack != "" {
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("stack=%s", w.stack))
	}

	return b.String()
}

// FromStatusError converts err to StatusError.
// 解析RPC返回的error, 如果是statusError转换而来, 则返回ok为true
func FromStatusError(err error) (statusErr *statusError, ok bool) {
	if err == nil {
		return nil, false
	}

	if se := GetStatusError(err); se != nil {
		return se, true
	}

	bizStatusErr, ok1 := kerrors.FromBizStatusError(err)
	if !ok1 {
		// 如果是框架异常，不做处理
		return nil, false
	}

	// statusErr通过Kitex业务异常进行服务间流转, 下面尝试从RPC返回的Kitex业务异常中解析statusErr
	code := bizStatusErr.BizStatusCode()
	msg := bizStatusErr.BizMessage()
	bizExtra := bizStatusErr.BizExtra()

	affectStability := true
	affectStabilityVal, ok2 := bizExtra[BizExtraKeyAffectStability]
	if ok2 && affectStabilityVal != "1" {
		affectStability = false
	}

	statusErr = &statusError{
		statusCode: code,
		message:    msg,
		ext: Extension{
			IsAffectStability: affectStability,
		},
	}

	customExtraVal, ok3 := bizExtra[BizExtraKeyCustomExtra]
	if ok3 {
		extra := map[string]string{}
		_ = sonic.UnmarshalString(customExtraVal, &extra)
		statusErr.ext.Extra = extra
	}

	return statusErr, true
}

// GetStatusError 获取错误链中最顶层的 StatusError.
// 如果有获取code或其他扩展字段的需求，再考虑对外暴露
func GetStatusError(err error) *statusError {
	if err == nil {
		return nil
	}

	var ws *statusError
	if errors.As(err, &ws) {
		return ws
	}

	return nil
}

type Option func(ws *withStatus)

func WithExtraMsg(extraMsg string) Option {
	return func(ws *withStatus) {
		if ws == nil || ws.status == nil || extraMsg == "" {
			return
		}
		ws.status.message = fmt.Sprintf("%s,%s", ws.status.message, extraMsg)
	}
}

func WithMsgParam(k, v string) Option {
	return func(ws *withStatus) {
		if ws == nil || ws.status == nil {
			return
		}
		ws.status.message = strings.ReplaceAll(ws.status.message, fmt.Sprintf("{%s}", k), v)
	}
}

func WithExtra(extra map[string]string) Option {
	return func(ws *withStatus) {
		if ws == nil || ws.status == nil || extra == nil {
			return
		}
		ws.status.ext.Extra = extra
	}
}

func NewByCode(code int32, options ...Option) *handlerErr {
	ws := &withStatus{
		status: getStatusByCode(code),
		cause:  nil,
		stack:  stack(),
	}

	for _, opt := range options {
		opt(ws)
	}

	return newHandlerErr(ws)
}

func WrapByCode(err error, code int32, options ...Option) *handlerErr {
	if err == nil {
		return nil
	}

	ws := &withStatus{
		status: getStatusByCode(code),
		cause:  err,
	}

	for _, opt := range options {
		opt(ws)
	}

	// skip if stack has already exist
	var stackTracer StackTracer
	if errors.As(err, &stackTracer) {
		return newHandlerErr(ws)
	}

	ws.stack = stack()

	return newHandlerErr(ws)
}

func newHandlerErr(ws *withStatus) *handlerErr {
	return &handlerErr{kitexBizErrWrapper: &kitexBizErrWrapper{withStatus: ws}}
}

func getStatusByCode(code int32) *statusError {
	codeDefinition, ok := CodeDefinitions[code]
	if ok {
		// predefined err code
		return &statusError{
			statusCode: code,
			message:    codeDefinition.Message,
			ext: Extension{
				IsAffectStability: codeDefinition.IsAffectStability,
			},
		}
	}

	return &statusError{
		statusCode: code,
		message:    DefaultErrorMsg,
		ext: Extension{
			IsAffectStability: DefaultIsAffectStability,
		},
	}
}
