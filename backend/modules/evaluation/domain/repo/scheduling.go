// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// CentralSchedulerRepo 中心调度所需的数据面读取接口。
//
// 权威源约定：
//   - 活跃实验、run 状态、item 状态全部来自 MySQL；
//   - 调度元数据来自 experiment.eval_conf.scheduling，属于 experiment 聚合；
//   - 本接口不写 DB，写状态仍走既有 item/run/实验服务。
type CentralSchedulerRepo interface {
	// ListActiveExperiments 扫描指定 owner env 下、调度方式为 central 的活跃实验。
	// ownerEnv 为空时返回空列表（归属不清不调度）。
	ListActiveExperiments(ctx context.Context, ownerEnv string, limit int) ([]*entity.CentralActiveExperiment, error)

	// ListRunItemStates 读取一次 run 的全部 item 状态，供缺口计算与结算使用。
	// 返回顺序按 item 落库顺序；调用方按状态自行筛选。
	ListRunItemStates(ctx context.Context, spaceID, exptID, exptRunID int64) ([]*entity.CentralItemState, error)

	// GetRunStatus 读取 run 当前状态；found=false 表示 run 不存在或已删除。
	GetRunStatus(ctx context.Context, spaceID, exptID, exptRunID int64) (status entity.ExptStatus, found bool, err error)
}
