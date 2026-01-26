// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
)

//go:generate mockgen -destination=mocks/auth_provider.go -package=mocks . IAuthProvider
type IAuthProvider interface {
	Authorization(ctx context.Context, param *AuthorizationParam) (err error)
	AuthorizationWithoutSPI(ctx context.Context, param *AuthorizationWithoutSPIParam) (err error)
	MAuthorizeWithoutSPI(ctx context.Context, spaceID int64, params []*AuthorizationWithoutSPIParam) error
}

type ActionObject struct {
	Action     *string
	EntityType *AuthEntityType
}

type AuthorizationParam struct {
	ObjectID      string
	SpaceID       int64
	ActionObjects []*ActionObject
}

type AuthorizationWithoutSPIParam struct {
	ObjectID      string
	SpaceID       int64
	ActionObjects []*ActionObject

	OwnerID         *string
	ResourceSpaceID int64
}

type AuthEntityType = string

const (
	AuthEntityType_Space = "Space"

	AuthEntityType_EvaluationExperiment = "EvaluationExperiment"

	AuthEntityType_EvaluationExptTemplate = "EvaluationExptTemplate"

	AuthEntityType_EvaluationSet = "EvaluationSet"

	AuthEntityType_Evaluator = "Evaluator"

	AuthEntityType_EvaluationTarget = "EvaluationTarget"
)
