// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IFluxWorkflowBindingRepo stores the service-owned copy identity in experiment.eval_conf.
type IFluxWorkflowBindingRepo interface {
	GetFluxWorkflowBinding(ctx context.Context, spaceID, exptID int64) (*entity.FluxWorkflowBinding, error)
	ReserveFluxWorkflowBinding(ctx context.Context, spaceID, exptID int64, pending *entity.FluxWorkflowBinding) (*entity.FluxWorkflowBinding, error)
	PutReadyFluxWorkflowBinding(ctx context.Context, spaceID, exptID int64, ready *entity.FluxWorkflowBinding) error
}

// IFluxItemRunRepo stores the mutable run index in eval_target_record.ext.
type IFluxItemRunRepo interface {
	GetFluxItemRun(ctx context.Context, spaceID, invokeID int64) (*entity.FluxItemRun, error)
	// CompareAndSwapFluxItemRun creates the state when expectedVersion is zero.
	// On a version conflict it returns updated=false and the latest state.
	CompareAndSwapFluxItemRun(ctx context.Context, spaceID, invokeID, expectedVersion int64, next *entity.FluxItemRun) (updated bool, current *entity.FluxItemRun, err error)
}
