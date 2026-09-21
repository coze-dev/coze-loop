// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package idlcontract

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
)

// This compile-only consumer implements exactly the previously published five methods.
type legacySPIImplementation struct{}

var _ spi.EvaluationSPIService = (*legacySPIImplementation)(nil)

func (*legacySPIImplementation) SearchEvalTarget(context.Context, *spi.SearchEvalTargetRequest) (*spi.SearchEvalTargetResponse, error) {
	panic("compile-only fixture")
}

func (*legacySPIImplementation) InvokeEvalTarget(context.Context, *spi.InvokeEvalTargetRequest) (*spi.InvokeEvalTargetResponse, error) {
	panic("compile-only fixture")
}

func (*legacySPIImplementation) AsyncInvokeEvalTarget(context.Context, *spi.AsyncInvokeEvalTargetRequest) (*spi.AsyncInvokeEvalTargetResponse, error) {
	panic("compile-only fixture")
}

func (*legacySPIImplementation) InvokeEvaluator(context.Context, *spi.InvokeEvaluatorRequest) (*spi.InvokeEvaluatorResponse, error) {
	panic("compile-only fixture")
}

func (*legacySPIImplementation) AsyncInvokeEvaluator(context.Context, *spi.AsyncInvokeEvaluatorRequest) (*spi.AsyncInvokeEvaluatorResponse, error) {
	panic("compile-only fixture")
}
