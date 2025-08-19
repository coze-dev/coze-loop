// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"time"
)

//go:generate mockgen -destination=mocks/http.go -package=mocks . IClient
type IClient interface {
	DoHTTPRequest(ctx context.Context, requestParam *RequestParam) error
}

type RequestParam struct {
	RequestURI string
	Method     string
	Header     map[string]string
	Body       interface{}
	Response   interface{}

	Timeout time.Duration
}
