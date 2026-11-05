// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// NewExptResultServiceWithPublication is the production Wire provider.
// NewExptResultService stays compatible with existing callers and legacy test doubles.
func NewExptResultServiceWithPublication(
	exptItemResultRepo repo.IExptItemResultRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	exptAnnotateRepo repo.IExptAnnotateRepo,
	exptStatsRepo repo.IExptStatsRepo,
	experimentRepo repo.IExperimentRepo,
	metric metrics.ExptMetric,
	lwt platestwrite.ILatestWriteTracker,
	idgen idgen.IIDGenerator,
	exptTurnResultFilterRepo repo.IExptTurnResultFilterRepo,
	evaluatorService EvaluatorService,
	evalTargetService IEvalTargetService,
	evalTargetRepo repo.IEvalTargetRepo,
	evaluationSetVersionService EvaluationSetVersionService,
	evaluationSetService IEvaluationSetService,
	evaluatorRecordService EvaluatorRecordService,
	evaluationSetItemService EvaluationSetItemService,
	publisher events.ExptEventPublisher,
	tagRPCAdapter rpc.ITagRPCAdapter,
	analysisService IEvaluationAnalysisService,
	fileProvider rpc.IFileProvider,
	scoreCalculator IEvaluatorScoreCalculator,
	// exptItemRefRepo ★ MultiSetConfig 实验按 item 回填 eval_set_id/eval_set_version_id 用。
	// variadic 保持既有调用方（含商业化 wire）向后兼容。
	resultPublishRepo repo.IExptResultPublishRepo,
	resultPublishLocker ManualRetryLockerFactory,
	exptItemRefRepo ...repo.IExptItemRefRepo,
) ExptResultService {
	result := NewExptResultService(exptItemResultRepo, exptTurnResultRepo, exptAnnotateRepo, exptStatsRepo, experimentRepo, metric, lwt, idgen, exptTurnResultFilterRepo, evaluatorService, evalTargetService, evalTargetRepo, evaluationSetVersionService, evaluationSetService, evaluatorRecordService, evaluationSetItemService, publisher, tagRPCAdapter, analysisService, fileProvider, scoreCalculator, exptItemRefRepo...).(*ExptResultServiceImpl)
	result.resultPublishRepo = resultPublishRepo
	result.resultPublishLocker = resultPublishLocker
	return result
}
