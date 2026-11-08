// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptRunTickLockerFactory func() lock.ILocker

type (
	admissionTickKey      struct{}
	admissionTickIdentity struct{ expt, run int64 }
	admissionTickOwnerKey struct{}
)

type admissionTickOwner struct {
	ctx  context.Context
	stop func()
}

func (e *ExptMangerImpl) newRunAdmissionTick(ctx context.Context, expt, space int64, mode entity.ExptRunMode) (context.Context, context.Context, func(), error) {
	nop := func() {}
	if err := ctx.Err(); err != nil {
		return ctx, ctx, nop, errno.WrapMQRetryErr(err)
	}
	if mode == entity.EvaluationModeAppend {
		return ctx, ctx, nop, nil
	}
	current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), expt, space)
	if err != nil {
		return ctx, ctx, nop, err
	}
	if current.ExptType != entity.ExptType_Offline {
		return ctx, ctx, nop, nil
	}
	if current.LatestRunID == 0 {
		return context.WithValue(ctx, admissionTickKey{}, admissionTickIdentity{expt, 0}), ctx, nop, nil
	}
	previous := current.LatestRunID
	// Caller cancellation stops writes; retain the tick through synchronous cleanup.
	parent, stopParent := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
	locker := e.runTickLocker()
	key := fmt.Sprintf("expt_run_exec_lock:%d:%d", expt, previous)
	held, leaseCtx, cancelLease, err := locker.LockWithRenew(parent, key, 5*time.Second, 5*time.Minute)
	if err != nil || !held {
		cancelLease()
		stopParent()
		if err == nil {
			err = errno.NewMQRetryErr("admission predecessor tick busy")
		}
		return ctx, ctx, nop, errno.WrapMQRetryErr(err)
	}
	stopLease := func() {
		cancelLease()
		if _, err := locker.Unlock(key); err != nil {
			logs.CtxWarn(ctx, "admission tick unlock fail, key: %v, err: %v", key, err)
		}
		stopParent()
	}
	latest, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(leaseCtx), expt, space)
	if err != nil || latest.LatestRunID != previous {
		stopLease()
		if err == nil {
			err = errno.NewMQRetryErr("admission current run changed")
		}
		return ctx, ctx, nop, err
	}
	opCtx, stopOp := context.WithCancel(ctx)
	unregister := context.AfterFunc(leaseCtx, stopOp)
	stop := func() { unregister(); stopOp(); stopLease() }
	if err := leaseCtx.Err(); err != nil {
		stop()
		return ctx, ctx, nop, errno.WrapMQRetryErr(err)
	}
	if err := opCtx.Err(); err != nil {
		stop()
		return ctx, ctx, nop, errno.WrapMQRetryErr(err)
	}
	opCtx = context.WithValue(opCtx, admissionTickKey{}, admissionTickIdentity{expt, previous})
	cleanupCtx := context.WithValue(contexts.WithCtxWriteDB(leaseCtx), admissionTickKey{}, admissionTickIdentity{expt, previous})
	cleanupCtx = context.WithValue(cleanupCtx, admissionTickOwnerKey{}, admissionTickOwner{ctx: leaseCtx, stop: stopLease})
	return contexts.WithCtxWriteDB(opCtx), cleanupCtx, stop, nil
}
