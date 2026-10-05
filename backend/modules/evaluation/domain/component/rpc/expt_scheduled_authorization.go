// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

var (
	ErrScheduledRunAuthorizationInvalid     = errors.New("invalid scheduled run authorization input")
	ErrScheduledRunAuthorizationUnsupported = errors.New("unsupported scheduled run permission mapping")
	ErrScheduledRunAuthorizationUnavailable = errors.New("scheduled run authorization unavailable")
	ErrScheduledRunAuthorizationDenied      = errors.New("scheduled run authorization denied")
)

// ScheduledRunAuthorizationResource uses existing evaluation Action/EntityType semantics.
// SpaceID is the resource's owning space, never a fallback to the run's space.
// Supply EvaluationSet/read and EvaluationTarget/run; unsupported mappings must fail closed.
type ScheduledRunAuthorizationResource struct {
	ObjectID   string
	SpaceID    int64
	EntityType AuthEntityType
	Action     string
}

// VerifiedScheduledRunBinding is internal input, not authentication proof by itself.
// The service must first verify the callback and exact active persisted binding, then
// load ALL template resources through authorized resolution (including cross-space grants).
// Resources must be non-nil; an explicit empty slice means the template has no resources.
// Create-experiment and template-read checks are implicit. Recheck binding CAS before submit.
type VerifiedScheduledRunBinding struct {
	Binding   *entity.ExptTemplateScheduleBinding
	Resources []ScheduledRunAuthorizationResource
}

// IScheduledRunAuthorizer is opt-in; it does not extend IAuthProvider or delegate the ctx user.
type IScheduledRunAuthorizer interface {
	AuthorizeScheduledRun(ctx context.Context, binding *VerifiedScheduledRunBinding) error
}
