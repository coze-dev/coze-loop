// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"sort"
	"strconv"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func isLegacyAsyncNotice(event *entity.ExptItemEvalEvent) bool {
	return event != nil && event.RequestID == "" && (event.ResumeSource != nil || event.AsyncReportTrigger || event.AsyncEvaluatorReportTrigger)
}

// The old codec's notice is not an attempt identity. Only a currently referenced,
// persisted invocation can authorize the local continuation under the item lock.
func (e *ExptItemEventEvalServiceImpl) bindLegacyAsyncNotice(ctx context.Context, event *entity.ExptItemEvalEvent, source *entity.ExptCurrentItemRunSource) (*entity.ExptItemEvalEvent, error) {
	local := event.Clone()
	local.RequestID, local.ExptRunMode, local.RetryTimes = source.Current.RequestID, source.Current.Mode, int(source.Current.RetryTimes)
	if local.ResumeSource == nil {
		ids := make([]int64, 0, len(source.Turns))
		for id := range source.Turns {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			turn := source.Turns[id]
			if turn == nil {
				return nil, errno.NewMQRetryErr("legacy resume turn missing")
			}
			if turn.Status == entity.TurnRunState_Success || turn.Status == entity.TurnRunState_Terminal {
				continue
			}
			if turn.TargetResultID <= 0 {
				return nil, errno.NewMQRetryErr("legacy resume target reference pending")
			}
			local.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: id, RecordID: turn.TargetResultID, InvokeID: strconv.FormatInt(turn.TargetResultID, 10)}
			if event.AsyncEvaluatorReportTrigger {
				recordIDs := legacyEvaluatorRecordIDs(turn.EvaluatorResultIds)
				if len(recordIDs) == 0 {
					return nil, errno.NewMQRetryErr("legacy evaluator references pending")
				}
				if e.evaluatorRecordService == nil {
					return nil, errno.NewMQRetryErr("legacy evaluator reader missing")
				}
				selected := int64(0)
				for _, recordID := range recordIDs {
					record, err := e.evaluatorRecordService.GetEvaluatorRecord(ctx, recordID, false)
					if err != nil {
						return nil, errno.WrapMQRetryErr(err)
					}
					if record == nil {
						return nil, errno.NewMQRetryErr("legacy evaluator record missing")
					}
					if record.Status != entity.EvaluatorRunStatusAsyncInvoking {
						selected = recordID
						break
					}
				}
				if selected == 0 {
					return nil, errno.NewMQRetryErr("legacy evaluator result pending")
				}
				local.ResumeSource = &entity.ExptResumeSource{Kind: "evaluator", TurnID: id, RecordID: selected, InvokeID: strconv.FormatInt(selected, 10)}
			}
			break
		}
		if local.ResumeSource == nil {
			// All turns may have committed before the old worker completed the item.
			for i := len(ids) - 1; i >= 0; i-- {
				turn := source.Turns[ids[i]]
				if turn != nil && turn.Status == entity.TurnRunState_Success && turn.TargetResultID > 0 {
					local.ResumeSource = &entity.ExptResumeSource{Kind: "target", TurnID: turn.TurnID, RecordID: turn.TargetResultID, InvokeID: strconv.FormatInt(turn.TargetResultID, 10)}
					break
				}
			}
		}
		if local.ResumeSource == nil {
			return nil, errno.NewMQRetryErr("legacy resume has no current invocation")
		}
	}
	turn := source.Turns[local.ResumeSource.TurnID]
	if turn != nil && local.ResumeSource.Kind == "target" && turn.TargetResultID == 0 {
		return nil, errno.NewMQRetryErr("legacy resume target reference pending")
	}
	valid, err := e.validateResumeSource(ctx, local, &source.Current)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, nil
	}
	ready, err := e.legacyResumeReady(ctx, local, &source.Current)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, errno.NewMQRetryErr("legacy resume result pending")
	}
	// A completed earlier turn must not restart another turn's live invocation.
	for id, other := range source.Turns {
		if id == local.ResumeSource.TurnID || other == nil || other.Status != entity.TurnRunState_Processing {
			continue
		}
		if other.TargetResultID > 0 {
			probe := local.CloneWithResumeSource("target", other.TargetResultID, id)
			valid, err := e.validateResumeSource(ctx, probe, &source.Current)
			if err != nil {
				return nil, err
			}
			if !valid {
				return nil, errno.NewMQRetryErr("other turn target binding changed")
			}
			ready, err := e.legacyResumeReady(ctx, probe, &source.Current)
			if err != nil {
				return nil, err
			}
			if !ready {
				return nil, errno.NewMQRetryErr("other turn target pending")
			}
		}
		for _, recordID := range legacyEvaluatorRecordIDs(other.EvaluatorResultIds) {
			probe := local.CloneWithResumeSource("evaluator", recordID, id)
			valid, err := e.validateResumeSource(ctx, probe, &source.Current)
			if err != nil {
				return nil, err
			}
			if !valid {
				return nil, errno.NewMQRetryErr("other turn evaluator binding changed")
			}
			ready, err := e.legacyResumeReady(ctx, probe, &source.Current)
			if err != nil {
				return nil, err
			}
			if !ready {
				return nil, errno.NewMQRetryErr("other turn evaluator pending")
			}
		}
	}
	local.AsyncReportTrigger = local.ResumeSource.Kind == "target"
	local.AsyncEvaluatorReportTrigger = local.ResumeSource.Kind == "evaluator"
	return local, nil
}

func legacyEvaluatorRecordIDs(refs *entity.EvaluatorResults) []int64 {
	if refs == nil {
		return nil
	}
	set := map[int64]bool{}
	for _, id := range refs.EvalVerIDToResID {
		if id > 0 {
			set[id] = true
		}
	}
	for _, ref := range refs.Registered {
		if ref != nil && ref.RecordID > 0 {
			set[ref.RecordID] = true
		}
	}
	for _, ref := range refs.Inline {
		if ref != nil && ref.RecordID > 0 {
			set[ref.RecordID] = true
		}
	}
	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (e *ExptItemEventEvalServiceImpl) legacyResumeReady(ctx context.Context, event *entity.ExptItemEvalEvent, current *entity.ExptItemCurrentRequest) (bool, error) {
	original := event.ResumeSource
	if original.Kind == "evaluator" {
		record, err := e.evaluatorRecordService.GetEvaluatorRecord(contexts.WithCtxWriteDB(ctx), original.RecordID, false)
		if err != nil {
			return false, errno.WrapMQRetryErr(err)
		}
		if record == nil {
			return false, errno.NewMQRetryErr("legacy evaluator record missing")
		}
		return record.Status == entity.EvaluatorRunStatusSuccess || record.Status == entity.EvaluatorRunStatusFail || record.Status == entity.EvaluatorRunStatusSkipped, nil
	}
	detail, err := e.manager.GetDetail(contexts.WithCtxWriteDB(ctx), event.ExptID, event.SpaceID, event.Session)
	if err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if detail == nil {
		return false, errno.NewMQRetryErr("legacy target resource missing")
	}
	space, err := e.resumeTargetSpace(ctx, event, detail)
	if err != nil {
		return false, err
	}
	record, err := e.evaTargetService.GetRecordByID(contexts.WithCtxWriteDB(ctx), space, original.RecordID)
	if err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if record == nil || record.Status == nil {
		return false, errno.NewMQRetryErr("legacy target record missing")
	}
	if record.ID != original.RecordID || record.SpaceID != space || record.ExperimentRunID != event.ExptRunID || record.ItemID != event.EvalSetItemID || record.TurnID != original.TurnID || record.ItemVersionID != 0 && record.ItemVersionID != current.ItemVersionID {
		return false, errno.NewMQRetryErr("legacy target resource binding changed")
	}
	version := detail.TargetVersionID
	if detail.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig && e.exptItemRefRepo != nil {
		ref, err := e.exptItemRefRepo.GetByExptIDAndItemID(contexts.WithCtxWriteDB(ctx), event.SpaceID, event.ExptID, event.EvalSetItemID)
		if err != nil {
			return false, errno.WrapMQRetryErr(err)
		}
		if ref != nil && ref.ItemConfig != nil && ref.ItemConfig.EvalTargetConf != nil && ref.ItemConfig.EvalTargetConf.TargetVersionID > 0 {
			version = ref.ItemConfig.EvalTargetConf.TargetVersionID
		}
	}
	if version > 0 && record.TargetVersionID != version {
		return false, errno.NewMQRetryErr("legacy target version changed")
	}
	status := gptr.Indirect(record.Status)
	return status == entity.EvalTargetRunStatusSuccess || status == entity.EvalTargetRunStatusFail, nil
}
