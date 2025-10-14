// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package errorx

import (
	"fmt"
	"strings"

	"github.com/coze-dev/coze-loop/backend/pkg/errorx/internal"
)

// StatusError is an interface for error with status code, you can
// create an error through NewByCode or WrapByCode and convert it back to
// StatusError through FromStatusError to obtain information such as
// error status code.
type StatusError interface {
	error
	Code() int32
	IsAffectStability() bool
	WithAffectStability(affectStability bool)
	Extra() map[string]string
	WithExtra(map[string]string)
}

// Option is used to configure an StatusError.
type Option = internal.Option

func WithExtraMsg(msg string) Option {
	return internal.WithExtraMsg(msg)
}

func WithMsgParam(k, v string) Option {
	return internal.WithMsgParam(k, v)
}

// WithExtra set extra for StatusError.
func WithExtra(extra map[string]string) Option {
	return internal.WithExtra(extra)
}

// NewByCode get an error predefined in the configuration file by statusCode
// with a stack trace at the point NewByCode is called.
func NewByCode(code int32, options ...Option) error {
	return internal.NewByCode(code, options...)
}

func New(format string, args ...any) error {
	return internal.WithStackTraceIfNotExists(fmt.Errorf(format, args...))
}

// WrapByCode returns an error annotating err with a stack trace
// at the point WrapByCode is called, and the status code.
func WrapByCode(err error, statusCode int32, options ...Option) error {
	if err == nil {
		return nil
	}

	return internal.WrapByCode(err, statusCode, options...)
}

// Wrapf returns an error annotating err with a stack trace
// at the point Wrapf is called, and the format specifier.
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}

	return internal.Wrapf(err, format, args...)
}

// FromStatusError converts err to StatusError.
func FromStatusError(err error) (statusErr StatusError, ok bool) {
	return internal.FromStatusError(err)
}

func ErrorWithoutStack(err error) string {
	if err == nil {
		return ""
	}
	errMsg := err.Error()
	if index := strings.Index(errMsg, "stack="); index != -1 {
		errMsg = errMsg[:index]
	}
	return errMsg
}
