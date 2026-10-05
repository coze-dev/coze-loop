// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"errors"
)

var (
	ErrScheduledCallbackUntrusted   = errors.New("untrusted scheduled callback")
	ErrScheduledCallbackUnavailable = errors.New("scheduled callback verification unavailable")
	ErrScheduledCallbackConfig      = errors.New("invalid scheduled callback verifier configuration")
)

// ScheduledCallbackVerifier is an optional internal RPC capability, not HTTP/user authentication.
// Success authenticates the service only. The application must still check the exact binding,
// job/version/enabled state, current bound-user permissions and durable instance deduplication.
type ScheduledCallbackVerifier interface {
	Verify(ctx context.Context) (*VerifiedScheduledCallback, error)
}

// VerifiedScheduledCallback contains owned, credential-free service metadata, never a delegated user.
// InstanceID is opaque scheduler deduplication metadata, not proof of job or binding ownership.
// Region/ExecutionScope describe the receiver deployment, not ticket user-data geography.
type VerifiedScheduledCallback struct {
	CallerPSM      string
	SignerPSM      string
	Method         string
	Namespace      string
	Region         string
	ExecutionScope string
	InstanceID     string
}
