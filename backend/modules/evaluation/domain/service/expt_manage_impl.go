// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/encoding"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/maps"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewExptManager(
	exptResultService ExptResultService,
	exptRepo repo.IExperimentRepo,
	exptRunLogRepo repo.IExptRunLogRepo,
	exptStatsRepo repo.IExptStatsRepo,
	exptItemResultRepo repo.IExptItemResultRepo,
	exptTurnResultRepo repo.IExptTurnResultRepo,
	configer component.IConfiger,
	quotaRepo repo.QuotaRepo,
	mutex lock.ILocker,
	idem idem.IdempotentService,
	publisher events.ExptEventPublisher,
	audit audit.IAuditService,
	idgen idgen.IIDGenerator,
	metric metrics.ExptMetric,
	lwt platestwrite.ILatestWriteTracker,
	evaluationSetVersionService EvaluationSetVersionService,
	evaluationSetService IEvaluationSetService,
	evalTargetService IEvalTargetService,
	evaluatorService EvaluatorService,
	benefitService benefit.IBenefitService,
	exptAggrResultService ExptAggrResultService,
	templateRepo repo.IExptTemplateRepo,
	templateManager IExptTemplateManager,
	notifyRPCAdapter rpc.INotifyRPCAdapter,
	userProvider rpc.IUserProvider,
) IExptManager {
	return &ExptMangerImpl{
		// tupleSvc:       tupleSvc,
		exptResultService:           exptResultService,
		exptRepo:                    exptRepo,
		runLogRepo:                  exptRunLogRepo,
		statsRepo:                   exptStatsRepo,
		itemResultRepo:              exptItemResultRepo,
		turnResultRepo:              exptTurnResultRepo,
		configer:                    configer,
		quotaRepo:                   quotaRepo,
		mutex:                       mutex,
		idem:                        idem,
		publisher:                   publisher,
		audit:                       audit,
		mtr:                         metric,
		idgenerator:                 idgen,
		lwt:                         lwt,
		evaluationSetVersionService: evaluationSetVersionService,
		evaluationSetService:        evaluationSetService,
		evalTargetService:           evalTargetService,
		evaluatorService:            evaluatorService,
		benefitService:              benefitService,
		exptAggrResultService:       exptAggrResultService,
		templateRepo:                templateRepo,
		templateManager:             templateManager,
		notifyRPCAdapter:            notifyRPCAdapter,
		userProvider:                userProvider,
	}
}

type ExptMangerImpl struct {
	// tupleSvc       IExptTupleService
	exptResultService           ExptResultService
	exptAggrResultService       ExptAggrResultService
	exptRepo                    repo.IExperimentRepo
	runLogRepo                  repo.IExptRunLogRepo
	statsRepo                   repo.IExptStatsRepo
	itemResultRepo              repo.IExptItemResultRepo
	turnResultRepo              repo.IExptTurnResultRepo
	quotaRepo                   repo.QuotaRepo
	mutex                       lock.ILocker
	idem                        idem.IdempotentService
	publisher                   events.ExptEventPublisher
	configer                    component.IConfiger
	audit                       audit.IAuditService
	mtr                         metrics.ExptMetric
	idgenerator                 idgen.IIDGenerator
	lwt                         platestwrite.ILatestWriteTracker
	evaluationSetVersionService EvaluationSetVersionService
	evaluationSetService        IEvaluationSetService
	evalTargetService           IEvalTargetService
	evaluatorService            EvaluatorService
	benefitService              benefit.IBenefitService
	templateRepo                repo.IExptTemplateRepo
	templateManager             IExptTemplateManager
	notifyRPCAdapter            rpc.INotifyRPCAdapter
	userProvider                rpc.IUserProvider
}

func (e *ExptMangerImpl) MGetDetail(ctx context.Context, exptIDs []int64, spaceID int64, session *entity.Session) ([]*entity.Experiment, error) {
	exptBasics, err := e.MGet(ctx, exptIDs, spaceID, session)
	if err != nil {
		return nil, err
	}

	exptDetails, err := e.packExperimentResult(ctx, exptBasics, spaceID, session)
	if err != nil {
		return nil, err
	}

	exptTuples, err := e.mGetTupleByExpt(ctx, exptDetails, spaceID, session)
	if err != nil {
		return nil, err
	}

	for idx := range exptTuples {
		exptDetails[idx].EvalSet = exptTuples[idx].EvalSet
		exptDetails[idx].Target = exptTuples[idx].Target
		exptDetails[idx].Evaluators = exptTuples[idx].Evaluators
	}

	return exptDetails, nil
}

func (e *ExptMangerImpl) GetDetail(ctx context.Context, exptID, spaceID int64, session *entity.Session, opts ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
	expt, err := e.Get(ctx, exptID, spaceID, session)
	if err != nil {
		return nil, err
	}

	// 填充 ExptTemplateMeta
	if err := e.fillExptTemplates(ctx, []*entity.Experiment{expt}, spaceID); err != nil {
		return nil, err
	}

	tuple, err := e.getTupleByExpt(ctx, expt, spaceID, session, opts...)
	if err != nil {
		return nil, err
	}
	expt.Evaluators = tuple.Evaluators
	expt.EvalSet = tuple.EvalSet
	expt.Target = tuple.Target

	expts, err := e.packExperimentResult(ctx, []*entity.Experiment{expt}, spaceID, session)
	if err != nil {
		return nil, err
	}

	return expts[0], nil
}

func (e *ExptMangerImpl) packExperimentResult(ctx context.Context, expts []*entity.Experiment, spaceID int64, session *entity.Session) ([]*entity.Experiment, error) {
	if len(expts) == 0 {
		return expts, nil
	}

	exptIDs := make([]int64, 0, len(expts))
	for _, expt := range expts {
		exptIDs = append(exptIDs, expt.ID)
	}

	stats, err := e.exptResultService.MGetStats(ctx, exptIDs, spaceID, session)
	if err != nil {
		return nil, err
	}
	exptID2Stats := gslice.ToMap(stats, func(t *entity.ExptStats) (int64, *entity.ExptStats) { return t.ExptID, t })

	for _, expt := range expts {
		expt.Stats = exptID2Stats[expt.ID]
	}

	aggrResults, err := e.exptAggrResultService.BatchGetExptAggrResultByExperimentIDs(ctx, spaceID, exptIDs)
	if err != nil {
		logs.CtxInfo(ctx, "BatchGetExptAggrResultByExperimentIDs fail, expt_ids: %v, err: %v", exptIDs, err)
	} else {
		arMemo := gslice.ToMap(aggrResults, func(t *entity.ExptAggregateResult) (int64, *entity.ExptAggregateResult) { return t.ExperimentID, t })
		for _, expt := range expts {
			expt.AggregateResult = arMemo[expt.ID]
		}
	}

	// 填充关联的实验模板基础信息
	if err := e.fillExptTemplates(ctx, expts, spaceID); err != nil {
		return nil, err
	}

	return expts, nil
}

// fillExptTemplates 为实验列表按需补充关联模板的基础信息（名称、描述等）
func (e *ExptMangerImpl) fillExptTemplates(ctx context.Context, expts []*entity.Experiment, spaceID int64) error {
	if len(expts) == 0 || e.templateRepo == nil {
		return nil
	}

	idSet := make(map[int64]struct{})
	for _, ex := range expts {
		if ex == nil || ex.ExptTemplateMeta == nil || ex.ExptTemplateMeta.ID == 0 {
			continue
		}
		idSet[ex.ExptTemplateMeta.ID] = struct{}{}
	}
	if len(idSet) == 0 {
		return nil
	}

	templateIDs := make([]int64, 0, len(idSet))
	for id := range idSet {
		templateIDs = append(templateIDs, id)
	}

	templates, err := e.templateRepo.MGetByID(ctx, templateIDs, spaceID)
	if err != nil {
		return err
	}
	if len(templates) == 0 {
		// 如果一个模板都查不到（例如都被软删除），将所有带模板 ID 的实验的 ExptTemplateMeta 置为 nil
		for _, ex := range expts {
			if ex == nil || ex.ExptTemplateMeta == nil || ex.ExptTemplateMeta.ID == 0 {
				continue
			}
			ex.ExptTemplateMeta = nil
		}
		return nil
	}

	templateMap := gslice.ToMap(templates, func(t *entity.ExptTemplate) (int64, *entity.ExptTemplate) {
		if t == nil {
			return 0, nil
		}
		return t.GetID(), t
	})

	for _, ex := range expts {
		if ex == nil || ex.ExptTemplateMeta == nil || ex.ExptTemplateMeta.ID == 0 {
			continue
		}
		if tpl, ok := templateMap[ex.ExptTemplateMeta.ID]; ok {
			// 只回填模板的 Meta 到 Experiment.ExptTemplateMeta，避免在 Experiment 上挂完整模板对象
			ex.ExptTemplateMeta = tpl.Meta
		} else {
			// 如果模板在数据库中查不到了（已被删除），将 ExptTemplateMeta 设置为 nil
			ex.ExptTemplateMeta = nil
		}
	}

	return nil
}

func (e *ExptMangerImpl) CheckName(ctx context.Context, name string, spaceID int64, session *entity.Session) (pass bool, err error) {
	_, exist, err := e.exptRepo.GetByName(ctx, name, spaceID)
	if err != nil {
		return false, err
	}
	if !exist {
		return true, nil
	}
	return false, nil
}

func (e *ExptMangerImpl) MDelete(ctx context.Context, exptIDs []int64, spaceID int64, session *entity.Session) error {
	logs.CtxInfo(ctx, "batch delete expts, expt_ids: %v", exptIDs)

	// 先获取实验信息，用于判断是否关联模板
	expts, err := e.exptRepo.MGetByID(ctx, exptIDs, spaceID)
	if err != nil {
		return err
	}

	// 批量删除实验
	if err := e.exptRepo.MDelete(ctx, exptIDs, spaceID); err != nil {
		return err
	}

	// 针对每个实验，如果关联了模板，则更新模板的 ExptInfo（删除实验，数量 -1）
	for _, expt := range expts {
		if expt == nil || expt.ExptTemplateMeta == nil || expt.ExptTemplateMeta.ID <= 0 || e.templateManager == nil {
			continue
		}
		if err := e.templateManager.UpdateExptInfo(ctx, expt.ExptTemplateMeta.ID, spaceID, expt.ID, expt.Status, -1); err != nil {
			// 记录错误但不影响主流程
			logs.CtxError(ctx, "[ExptEval] UpdateExptInfo failed in MDelete, template_id: %v, expt_id: %v, err: %v",
				expt.ExptTemplateMeta.ID, expt.ID, err)
		}
	}

	return nil
}

func (e *ExptMangerImpl) makeExptMutexLockKey(exptID int64) string {
	return fmt.Sprintf("expt_run_mutex_lock:%d", exptID)
}

func (e *ExptMangerImpl) makeExptCompletingLockKey(exptID, exptRunID int64) string {
	return fmt.Sprintf("expt_completing_mutex_lock:%d:%d", exptID, exptRunID)
}

func (e *ExptMangerImpl) getTupleByExpt(ctx context.Context, expt *entity.Experiment, spaceID int64, session *entity.Session, opts ...entity.GetExptTupleOptionFn) (*entity.ExptTuple, error) {
	return e.getExptTupleByID(ctx, e.packTupleID(ctx, expt), spaceID, session, opts...)
}

func (e *ExptMangerImpl) mGetTupleByExpt(ctx context.Context, expts []*entity.Experiment, spaceID int64, session *entity.Session, opts ...entity.GetExptTupleOptionFn) ([]*entity.ExptTuple, error) {
	tupleIDs := make([]*entity.ExptTupleID, 0, len(expts))
	for _, exptDO := range expts {
		tupleIDs = append(tupleIDs, e.packTupleID(ctx, exptDO))
	}
	exptTuples, err := e.mgetExptTupleByID(ctx, tupleIDs, spaceID, session, opts...)
	if err != nil {
		return nil, err
	}

	return exptTuples, nil
}

func (e *ExptMangerImpl) getExptTupleByID(ctx context.Context, exptTupleID *entity.ExptTupleID, spaceID int64, session *entity.Session, opts ...entity.GetExptTupleOptionFn) (*entity.ExptTuple, error) {
	var (
		target     *entity.EvalTarget
		evalSet    *entity.EvaluationSet
		evaluators []*entity.Evaluator
	)
	pool, err := goroutine.NewPool(3)
	if err != nil {
		return nil, err
	}

	if exptTupleID.VersionedTargetID != nil {
		pool.Add(func() error {
			var poolErr error
			target, poolErr = e.evalTargetService.GetEvalTargetVersion(ctx, spaceID, exptTupleID.VersionedTargetID.VersionID, true)
			if poolErr != nil {
				return poolErr
			}
			return nil
		})
	}

	if exptTupleID.VersionedEvalSetID != nil {
		if exptTupleID.VersionedEvalSetID.EvalSetID != exptTupleID.VersionedEvalSetID.VersionID {
			pool.Add(func() error {
				version, set, poolErr := e.evaluationSetVersionService.GetEvaluationSetVersion(ctx, spaceID, exptTupleID.VersionedEvalSetID.VersionID, gptr.Of(true))
				if poolErr != nil {
					return poolErr
				}
				set.EvaluationSetVersion = version
				evalSet = set
				return nil
			})
		} else {
			pool.Add(func() error {
				var poolErr error
				evalSet, poolErr = e.evaluationSetService.GetEvaluationSet(ctx, gptr.Of(spaceID), exptTupleID.VersionedEvalSetID.EvalSetID, gptr.Of(false))
				if poolErr != nil {
					return poolErr
				}
				return nil
			})
		}
	}

	if len(exptTupleID.EvaluatorVersionIDs) > 0 {
		pool.Add(func() error {
			var poolErr error
			evaluators, poolErr = e.evaluatorService.BatchGetEvaluatorVersion(ctx, nil, exptTupleID.EvaluatorVersionIDs, false)
			if poolErr != nil {
				return poolErr
			}
			return nil
		})
	}

	if err := pool.Exec(ctx); err != nil { // ignore_security_alert_wait_for_fix SQL_INJECTION
		return nil, err
	}

	return &entity.ExptTuple{
		Target:     target,
		EvalSet:    evalSet,
		Evaluators: evaluators,
	}, nil
}

func (e *ExptMangerImpl) mgetExptTupleByID(ctx context.Context, tupleIDs []*entity.ExptTupleID, spaceID int64, session *entity.Session, opts ...entity.GetExptTupleOptionFn) ([]*entity.ExptTuple, error) {
	var (
		versionedTargetIDs  = make([]*entity.VersionedTargetID, 0, len(tupleIDs))
		versionedEvalSetIDs = make([]*entity.VersionedEvalSetID, 0, len(tupleIDs))
		evaluatorVersionIDs []int64

		targets    []*entity.EvalTarget
		evalSet    []*entity.EvaluationSet
		evaluators []*entity.Evaluator
	)

	for _, etids := range tupleIDs {
		versionedEvalSetIDs = append(versionedEvalSetIDs, etids.VersionedEvalSetID)
		if etids.VersionedTargetID != nil {
			versionedTargetIDs = append(versionedTargetIDs, etids.VersionedTargetID)
		}
		if len(etids.EvaluatorVersionIDs) > 0 {
			evaluatorVersionIDs = append(evaluatorVersionIDs, etids.EvaluatorVersionIDs...)
		}
	}

	pool, err := goroutine.NewPool(3)
	if err != nil {
		return nil, err
	}

	if len(versionedTargetIDs) > 0 {
		pool.Add(func() error {
			// 去重,可以优化循环次数
			targetVersionIDs := make([]int64, 0, len(versionedTargetIDs))
			for _, tids := range versionedTargetIDs {
				targetVersionIDs = append(targetVersionIDs, tids.VersionID)
			}
			targetVersionIDs = maps.ToSlice(gslice.ToMap(targetVersionIDs, func(t int64) (int64, bool) { return t, true }), func(k int64, v bool) int64 { return k })
			var poolErr error
			targets, poolErr = e.evalTargetService.BatchGetEvalTargetVersion(ctx, spaceID, targetVersionIDs, true)
			if poolErr != nil {
				return poolErr
			}
			return nil
		})
	}

	if len(versionedEvalSetIDs) > 0 {
		evalSetVersionIDs := make([]int64, 0, len(versionedEvalSetIDs))
		for _, ids := range versionedEvalSetIDs {
			if ids.EvalSetID != ids.VersionID {
				evalSetVersionIDs = append(evalSetVersionIDs, ids.VersionID)
			}
		}
		if len(evalSetVersionIDs) > 0 {
			pool.Add(func() error {
				verIDs := maps.ToSlice(gslice.ToMap(evalSetVersionIDs, func(t int64) (int64, bool) { return t, true }), func(k int64, v bool) int64 { return k })
				got, poolErr := e.evaluationSetVersionService.BatchGetEvaluationSetVersions(ctx, gptr.Of(spaceID), verIDs, gptr.Of(true))
				if poolErr != nil {
					return poolErr
				}
				for _, elem := range got {
					if elem == nil {
						continue
					}
					elem.EvaluationSet.EvaluationSetVersion = elem.Version
					evalSet = append(evalSet, elem.EvaluationSet)
				}
				return nil
			})
		}
		// 草稿的evalSetID和versionID相同
		evalSetIDs := make([]int64, 0, len(versionedEvalSetIDs))
		for _, ids := range versionedEvalSetIDs {
			if ids.EvalSetID == ids.VersionID {
				evalSetIDs = append(evalSetIDs, ids.EvalSetID)
			}
		}
		if len(evalSetIDs) > 0 {
			pool.Add(func() error {
				setIDs := maps.ToSlice(gslice.ToMap(evalSetIDs, func(t int64) (int64, bool) { return t, true }), func(k int64, v bool) int64 { return k })
				got, poolErr := e.evaluationSetService.BatchGetEvaluationSets(ctx, gptr.Of(spaceID), setIDs, gptr.Of(false))
				if poolErr != nil {
					return poolErr
				}
				for _, elem := range got {
					if elem == nil {
						continue
					}
					evalSet = append(evalSet, elem)
				}
				return nil
			})
		}
	}

	if len(evaluatorVersionIDs) > 0 {
		pool.Add(func() error {
			var poolErr error
			evaluators, poolErr = e.evaluatorService.BatchGetEvaluatorVersion(ctx, nil, evaluatorVersionIDs, true)
			if poolErr != nil {
				return poolErr
			}
			return nil
		})
	}

	if err := pool.Exec(ctx); err != nil { // ignore_security_alert_wait_for_fix SQL_INJECTION
		return nil, err
	}

	targetMap := gslice.ToMap(targets, func(t *entity.EvalTarget) (int64, *entity.EvalTarget) {
		if t == nil || t.EvalTargetVersion == nil {
			return 0, nil
		}
		return t.EvalTargetVersion.ID, t
	})
	evalSetMap := gslice.ToMap(evalSet, func(t *entity.EvaluationSet) (int64, *entity.EvaluationSet) {
		if t == nil || t.EvaluationSetVersion == nil {
			return 0, nil
		}
		return t.EvaluationSetVersion.ID, t
	})
	evaluatorMap := gslice.ToMap(evaluators, func(t *entity.Evaluator) (int64, *entity.Evaluator) {
		return t.GetEvaluatorVersionID(), t
	})

	res := make([]*entity.ExptTuple, 0, len(tupleIDs))
	for _, tupleIDs := range tupleIDs {
		// cevaluators := make([]*entity.Evaluator, 0, len(tupleIDs.EvaluatorVersionIDs))
		// for _, evaluatorVersionID := range tupleIDs.EvaluatorVersionIDs {
		//	cevaluators = append(cevaluators, evaluatorMap[evaluatorVersionID])
		// }
		tuple := &entity.ExptTuple{
			EvalSet: evalSetMap[tupleIDs.VersionedEvalSetID.VersionID],
			// Evaluators: cevaluators,
		}
		if tupleIDs.VersionedTargetID != nil {
			tuple.Target = targetMap[tupleIDs.VersionedTargetID.VersionID]
		}
		if len(tupleIDs.EvaluatorVersionIDs) > 0 {
			cevaluators := make([]*entity.Evaluator, 0, len(tupleIDs.EvaluatorVersionIDs))
			for _, evaluatorVersionID := range tupleIDs.EvaluatorVersionIDs {
				cevaluators = append(cevaluators, evaluatorMap[evaluatorVersionID])
			}
			tuple.Evaluators = cevaluators
		}
		res = append(res, tuple)
	}

	return res, nil
}

func (e *ExptMangerImpl) packTupleID(ctx context.Context, expt *entity.Experiment) *entity.ExptTupleID {
	// evaluatorVersionIDs := make([]int64, 0, len(expt.EvaluatorVersionRef))
	// for _, ref := range expt.EvaluatorVersionRef {
	//	evaluatorVersionIDs = append(evaluatorVersionIDs, ref.EvaluatorVersionID)
	// }

	exptTupleID := &entity.ExptTupleID{
		VersionedEvalSetID: &entity.VersionedEvalSetID{
			EvalSetID: expt.EvalSetID,
			VersionID: expt.EvalSetVersionID,
		},
		// EvaluatorVersionIDs: evaluatorVersionIDs,
	}

	if expt.TargetID > 0 || expt.TargetVersionID > 0 {
		exptTupleID.VersionedTargetID = &entity.VersionedTargetID{
			TargetID:  expt.TargetID,
			VersionID: expt.TargetVersionID,
		}
	}

	if len(expt.EvaluatorVersionRef) > 0 {
		evaluatorVersionIDs := make([]int64, 0, len(expt.EvaluatorVersionRef))
		for _, ref := range expt.EvaluatorVersionRef {
			evaluatorVersionIDs = append(evaluatorVersionIDs, ref.EvaluatorVersionID)
		}
		exptTupleID.EvaluatorVersionIDs = evaluatorVersionIDs
	}

	return exptTupleID
}

func (e *ExptMangerImpl) CreateExpt(ctx context.Context, req *entity.CreateExptParam, session *entity.Session) (*entity.Experiment, error) {
	if req.ExptType == entity.ExptType_Online && req.CreateEvalTargetParam != nil {
		req.CreateEvalTargetParam.SourceTargetVersion = gptr.Of(consts.DefaultSourceTargetVersion)
	}

	var versionedTargetID *entity.VersionedTargetID
	if !req.CreateEvalTargetParam.IsNull() {
		opts := make([]entity.Option, 0)
		opts = append(opts, entity.WithCozeBotPublishVersion(req.CreateEvalTargetParam.BotPublishVersion),
			entity.WithCozeBotInfoType(gptr.Indirect(req.CreateEvalTargetParam.BotInfoType)),
			entity.WithRegion(req.CreateEvalTargetParam.Region),
			entity.WithEnv(req.CreateEvalTargetParam.Env))
		if req.CreateEvalTargetParam.CustomEvalTarget != nil {
			opts = append(opts, entity.WithCustomEvalTarget(&entity.CustomEvalTarget{
				ID:        req.CreateEvalTargetParam.CustomEvalTarget.ID,
				Name:      req.CreateEvalTargetParam.CustomEvalTarget.Name,
				AvatarURL: req.CreateEvalTargetParam.CustomEvalTarget.AvatarURL,
				Ext:       req.CreateEvalTargetParam.CustomEvalTarget.Ext,
			}))
		}
		targetID, targetVersionID, err := e.evalTargetService.CreateEvalTarget(ctx, req.WorkspaceID, gptr.Indirect(req.CreateEvalTargetParam.SourceTargetID), gptr.Indirect(req.CreateEvalTargetParam.SourceTargetVersion), gptr.Indirect(req.CreateEvalTargetParam.EvalTargetType),
			opts...)
		if err != nil {
			return nil, errorx.Wrapf(err, "CreateEvalTarget failed, param: %v", json.Jsonify(req.CreateEvalTargetParam))
		}

		versionedTargetID = &entity.VersionedTargetID{
			TargetID:  targetID,
			VersionID: targetVersionID,
		}
	} else if req.TargetID != nil && *req.TargetID > 0 && req.TargetVersionID > 0 {
		// 使用已有 target（如从模板提交实验时）
		versionedTargetID = &entity.VersionedTargetID{
			TargetID:  *req.TargetID,
			VersionID: req.TargetVersionID,
		}
	}

	tuple, err := e.getExptTupleByID(ctx, &entity.ExptTupleID{
		VersionedEvalSetID: &entity.VersionedEvalSetID{
			EvalSetID: req.EvalSetID,
			VersionID: req.EvalSetVersionID,
		},
		VersionedTargetID:   versionedTargetID,
		EvaluatorVersionIDs: req.EvaluatorVersionIds,
	}, req.WorkspaceID, session)
	if err != nil {
		return nil, err
	}

	ids, err := e.idgenerator.GenMultiIDs(ctx, 2)
	if err != nil {
		return nil, err
	}

	evaluatorRefs := make([]*entity.ExptEvaluatorVersionRef, 0)
	exptTurnResultFilterKeyMappings := make([]*entity.ExptTurnResultFilterKeyMapping, 0)
	for i, es := range tuple.Evaluators {
		evaluatorRefs = append(evaluatorRefs, &entity.ExptEvaluatorVersionRef{
			EvaluatorID:        es.ID,
			EvaluatorVersionID: es.GetEvaluatorVersionID(),
		})
		exptTurnResultFilterKeyMappings = append(exptTurnResultFilterKeyMappings, &entity.ExptTurnResultFilterKeyMapping{
			SpaceID:   req.WorkspaceID,
			ExptID:    ids[0],
			FromField: strconv.FormatInt(es.GetEvaluatorVersionID(), 10),
			ToKey:     "key" + strconv.Itoa(i+1),
			FieldType: entity.FieldTypeEvaluator,
		})
	}

	do := &entity.Experiment{
		ID:                  ids[0],
		SpaceID:             req.WorkspaceID,
		CreatedBy:           session.UserID,
		Name:                req.Name,
		Description:         req.Desc,
		EvalSetVersionID:    req.EvalSetVersionID,
		EvalSetID:           req.EvalSetID,
		EvaluatorVersionRef: evaluatorRefs,
		EvalConf:            req.ExptConf,
		Status:              entity.ExptStatus_Pending,
		StartAt:             gptr.Of(time.Now()),
		ExptType:            req.ExptType,
		MaxAliveTime:        req.MaxAliveTime,
		SourceType:          req.SourceType,
		SourceID:            req.SourceID,

		Target:     tuple.Target,
		Evaluators: tuple.Evaluators,
		EvalSet:    tuple.EvalSet,
	}

	// 如果提供了模板 ID，设置 ExptTemplateMeta
	if req.ExptTemplateID > 0 {
		do.ExptTemplateMeta = &entity.ExptTemplateMeta{
			ID: req.ExptTemplateID,
		}
	}

	// 根据 EvaluatorConf.ScoreWeight 设置实验是否启用分数权重
	if do.EvalConf != nil && do.EvalConf.ConnectorConf.EvaluatorsConf != nil {
		do.EvalConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight = false
		for _, ec := range do.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConf {
			if ec != nil && ec.ScoreWeight != nil && *ec.ScoreWeight > 0 {
				do.EvalConf.ConnectorConf.EvaluatorsConf.EnableScoreWeight = true
				break
			}
		}
	}

	if versionedTargetID != nil {
		do.TargetID = versionedTargetID.TargetID
		do.TargetVersionID = versionedTargetID.VersionID
		if !req.CreateEvalTargetParam.IsNull() {
			do.TargetType = gptr.Indirect(req.CreateEvalTargetParam.EvalTargetType)
		} else if tuple.Target != nil {
			if tuple.Target.EvalTargetVersion != nil {
				do.TargetType = tuple.Target.EvalTargetVersion.EvalTargetType
			} else {
				do.TargetType = tuple.Target.EvalTargetType
			}
		}
		if do.EvalConf != nil && do.EvalConf.ConnectorConf.TargetConf != nil {
			do.EvalConf.ConnectorConf.TargetConf.TargetVersionID = do.TargetVersionID
		}
	}

	err = e.CheckRun(ctx, do, req.WorkspaceID, session, entity.WithCheckBenefit())
	if err != nil {
		return nil, err
	}

	stats := &entity.ExptStats{
		ID:      ids[1],
		SpaceID: req.WorkspaceID,
		ExptID:  do.ID,
	}
	if err := e.exptResultService.CreateStats(ctx, stats, session); err != nil {
		return nil, err
	}

	if err := e.exptResultService.InsertExptTurnResultFilterKeyMappings(ctx, exptTurnResultFilterKeyMappings); err != nil {
		return nil, err
	}

	if err := e.Create(ctx, do, session); err != nil {
		return nil, err
	}

	return do, nil
}

func (e *ExptMangerImpl) Create(ctx context.Context, expt *entity.Experiment, session *entity.Session) error {
	refs := expt.ToEvaluatorRefDO()

	pass, err := e.CheckName(ctx, expt.Name, expt.SpaceID, session)
	if !pass {
		return errorx.NewByCode(errno.ExperimentNameExistedCode, errorx.WithExtraMsg(fmt.Sprintf("name %s", expt.Name)))
	}
	if err != nil {
		return err
	}

	if err = e.exptRepo.Create(ctx, expt, refs); err != nil {
		return err
	}

	e.lwt.SetWriteFlag(ctx, platestwrite.ResourceTypeExperiment, expt.ID)

	return nil
}

func (e *ExptMangerImpl) Get(ctx context.Context, exptID, spaceID int64, session *entity.Session) (*entity.Experiment, error) {
	expts, err := e.MGet(ctx, []int64{exptID}, spaceID, session)
	if err != nil {
		return nil, err
	}

	if len(expts) == 0 {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("experiment %d not found", exptID)))
	}
	got := expts[0]
	if got == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("experiment %d not found", exptID)))
	}

	return got, nil
}

func (e *ExptMangerImpl) MGet(ctx context.Context, exptIDs []int64, spaceID int64, session *entity.Session) ([]*entity.Experiment, error) {
	if len(exptIDs) == 1 && e.lwt.CheckWriteFlagByID(ctx, platestwrite.ResourceTypeExperiment, exptIDs[0]) {
		ctx = contexts.WithCtxWriteDB(ctx)
	}

	expts, err := e.exptRepo.MGetByID(ctx, exptIDs, spaceID)
	if err != nil {
		return nil, err
	}

	return expts, nil
}

func (e *ExptMangerImpl) List(ctx context.Context, page, pageSize int32, spaceID int64, filter *entity.ExptListFilter, orderBys []*entity.OrderBy, session *entity.Session) ([]*entity.Experiment, int64, error) {
	expts, count, err := e.exptRepo.List(ctx, page, pageSize, filter, orderBys, spaceID)
	if err != nil {
		return nil, 0, err
	}
	tupleIDs := make([]*entity.ExptTupleID, 0, len(expts))
	for _, exptDO := range expts {
		tupleIDs = append(tupleIDs, e.packTupleID(ctx, exptDO))
	}
	exptTuples, err := e.mgetExptTupleByID(ctx, tupleIDs, spaceID, session)
	if err != nil {
		return nil, 0, err
	}
	for idx := range exptTuples {
		if expts[idx].ExptType != entity.ExptType_Online {
			expts[idx].EvalSet = exptTuples[idx].EvalSet
		}
		expts[idx].Target = exptTuples[idx].Target
		expts[idx].Evaluators = exptTuples[idx].Evaluators
	}

	expts, err = e.packExperimentResult(ctx, expts, spaceID, session)
	if err != nil {
		return nil, 0, err
	}

	return expts, count, nil
}

func (e *ExptMangerImpl) ListExptRaw(ctx context.Context, page, pageSize int32, spaceID int64, filter *entity.ExptListFilter) ([]*entity.Experiment, int64, error) {
	expts, total, err := e.exptRepo.List(ctx, page, pageSize, filter, nil, spaceID)
	if err != nil {
		return nil, 0, err
	}
	return expts, total, nil
}

func (e *ExptMangerImpl) Update(ctx context.Context, expt *entity.Experiment, session *entity.Session) error {
	data := map[string]string{
		"texts": strings.Join([]string{expt.Name, expt.Description}, ","),
	}
	record, err := e.audit.Audit(ctx, audit.AuditParam{
		ObjectID:  expt.ID,
		AuditType: audit.AuditType_CozeLoopExptModify,
		AuditData: data,
		ReqID:     encoding.Encode(ctx, data),
	})
	if err != nil {
		logs.CtxError(ctx, "audit: failed to audit, err=%v", err) // 审核服务不可用，默认通过
	}
	if record.AuditStatus == audit.AuditStatus_Rejected {
		return errorx.NewByCode(errno.RiskContentDetectedCode)
	}

	return e.exptRepo.Update(ctx, expt)
}

func (e *ExptMangerImpl) Delete(ctx context.Context, exptID, spaceID int64, session *entity.Session) error {
	logs.CtxInfo(ctx, "delete expt, expt_id: %v", exptID)

	// 先获取实验信息，用于判断是否关联模板
	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return err
	}

	// 删除实验
	if err := e.exptRepo.Delete(ctx, exptID, spaceID); err != nil {
		return err
	}

	// 如果实验关联了模板，更新模板的 ExptInfo（删除实验，数量 -1）
	if expt != nil && expt.ExptTemplateMeta != nil && expt.ExptTemplateMeta.ID > 0 && e.templateManager != nil {
		if err := e.templateManager.UpdateExptInfo(ctx, expt.ExptTemplateMeta.ID, spaceID, exptID, expt.Status, -1); err != nil {
			// 记录错误但不影响主流程
			logs.CtxError(ctx, "[ExptEval] UpdateExptInfo failed in Delete, template_id: %v, expt_id: %v, err: %v",
				expt.ExptTemplateMeta.ID, exptID, err)
		}
	}

	return nil
}

func (e *ExptMangerImpl) Clone(ctx context.Context, exptID, spaceID int64, session *entity.Session) (*entity.Experiment, error) {
	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return nil, err
	}

	id, err := e.idgenerator.GenID(ctx)
	if err != nil {
		return nil, err
	}

	expt.ID = id

	return expt, e.Create(ctx, expt, session)
}
