// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package application

import (
	"context"
	"reflect"
	"strconv"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	exptdomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func validateStandardEvalOutputSourceRequest(req *expt.MGetExperimentStandardEvalOutputsRequest) (bool, error) {
	run, request, records := req.ExpectedExptRunID != nil, req.ExpectedRequestID != nil, req.ExpectedSourceTurnRecords != nil
	expected := run || request || records
	invalid := func() (bool, error) {
		return false, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("expected source must be complete, canonical and ordered; expected queries require one item"))
	}
	if expected {
		if !run || !request || !records || req.GetExpectedExptRunID() <= 0 || len(req.ExpectedSourceTurnRecords) == 0 || len(req.GetItemIds()) != 1 {
			return invalid()
		}
		if _, err := standardEvalOutputExpectedRecords(req.ExpectedSourceTurnRecords); err != nil {
			return invalid()
		}
	}
	if expected || req.GetSourceOnly() {
		if req.GetWorkspaceID() <= 0 || req.GetExptID() <= 0 {
			return invalid()
		}
		seen := map[int64]bool{}
		for _, item := range req.GetItemIds() {
			if item <= 0 || seen[item] {
				return invalid()
			}
			seen[item] = true
		}
	}
	return expected, nil
}

func standardSourceDecimal(value *string, positive bool) (int64, bool) {
	if value == nil {
		return 0, false
	}
	n, err := strconv.ParseInt(*value, 10, 64)
	return n, err == nil && n >= 0 && (!positive || n > 0) && strconv.FormatInt(n, 10) == *value
}

func standardEvalOutputExpectedRecords(input []*expt.ItemStandardEvalOutputTurnSource) ([]component.ItemCompleteTurnSource, error) {
	invalid := errorx.NewByCode(errno.CommonInvalidParamCode)
	turns := make([]component.ItemCompleteTurnSource, 0, len(input))
	previousTurn := int64(-1)
	for _, turn := range input {
		if turn == nil {
			return nil, invalid
		}
		id, ok := standardSourceDecimal(turn.TurnID, false)
		if !ok || id <= previousTurn {
			return nil, invalid
		}
		if _, ok := standardSourceDecimal(turn.TargetRecordID, false); !ok {
			return nil, invalid
		}
		previousTurn = id
		entry := component.ItemCompleteTurnSource{TurnID: *turn.TurnID, TargetRecordID: *turn.TargetRecordID}
		previousVersion, previousAlias := int64(-1), ""
		for _, ref := range turn.Registered {
			if ref == nil || ref.Alias == nil {
				return nil, invalid
			}
			version, ok := standardSourceDecimal(ref.VersionID, true)
			if !ok || version < previousVersion || version == previousVersion && *ref.Alias <= previousAlias {
				return nil, invalid
			}
			if _, ok := standardSourceDecimal(ref.RecordID, true); !ok {
				return nil, invalid
			}
			previousVersion, previousAlias = version, *ref.Alias
			entry.Registered = append(entry.Registered, component.ItemCompleteRegisteredSource{VersionID: *ref.VersionID, Alias: *ref.Alias, RecordID: *ref.RecordID})
		}
		previousInline := ""
		for _, ref := range turn.Inline {
			if ref == nil || ref.InlineKey == nil || *ref.InlineKey == "" || *ref.InlineKey <= previousInline {
				return nil, invalid
			}
			if _, ok := standardSourceDecimal(ref.RecordID, true); !ok {
				return nil, invalid
			}
			previousInline = *ref.InlineKey
			entry.Inline = append(entry.Inline, component.ItemCompleteInlineSource{InlineKey: *ref.InlineKey, RecordID: *ref.RecordID})
		}
		turns = append(turns, entry)
	}
	return turns, nil
}

func standardEvalOutputSourceRecordsDTO(records []component.ItemCompleteTurnSource) []*expt.ItemStandardEvalOutputTurnSource {
	result := make([]*expt.ItemStandardEvalOutputTurnSource, 0, len(records))
	for _, turn := range records {
		entry := &expt.ItemStandardEvalOutputTurnSource{TurnID: gptr.Of(turn.TurnID), TargetRecordID: gptr.Of(turn.TargetRecordID)}
		for _, ref := range turn.Registered {
			entry.Registered = append(entry.Registered, &expt.ItemStandardEvalOutputRegisteredSource{VersionID: gptr.Of(ref.VersionID), Alias: gptr.Of(ref.Alias), RecordID: gptr.Of(ref.RecordID)})
		}
		for _, ref := range turn.Inline {
			entry.Inline = append(entry.Inline, &expt.ItemStandardEvalOutputInlineSource{InlineKey: gptr.Of(ref.InlineKey), RecordID: gptr.Of(ref.RecordID)})
		}
		result = append(result, entry)
	}
	return result
}

func (e *experimentApplication) mgetStandardEvalOutputsWithSource(ctx context.Context, req *expt.MGetExperimentStandardEvalOutputsRequest, expected bool) (*expt.MGetExperimentStandardEvalOutputsResponse, error) {
	port, ok := e.resultSvc.(service.ExptStandardEvalOutputSourceService)
	if !ok {
		return nil, errorx.NewByCode(errno.StandardEvalOutputSourceNotReadyCode)
	}
	items := make([]*expt.ItemStandardEvalOutput, 0, len(req.GetItemIds()))
	for _, item := range req.GetItemIds() {
		run := int64(0)
		if expected {
			run = req.GetExpectedExptRunID()
		}
		frozen, err := port.GetStandardEvalOutputSource(ctx, req.GetWorkspaceID(), req.GetExptID(), run, item, req.ExpectedRequestID)
		if err != nil {
			return nil, err
		}
		if frozen == nil || frozen.Snapshot == nil || frozen.Snapshot.Source == nil {
			return nil, errorx.NewByCode(errno.StandardEvalOutputSourceNotReadyCode)
		}
		records, err := service.StandardEvalOutputSourceRecords(frozen)
		if err != nil {
			return nil, err
		}
		c := frozen.Snapshot.Source.Current
		if c.SpaceID != req.GetWorkspaceID() || c.ExptID != req.GetExptID() || c.ItemID != item || expected && (c.RunID != run || c.RequestID != req.GetExpectedRequestID()) {
			return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
		}
		if expected {
			wanted, err := standardEvalOutputExpectedRecords(req.ExpectedSourceTurnRecords)
			if err != nil || !reflect.DeepEqual(records, wanted) {
				return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
			}
		}
		out := &expt.ItemStandardEvalOutput{ExptID: gptr.Of(c.ExptID), ItemID: gptr.Of(c.ItemID), Status: gptr.Of(exptdomain.ItemRunState_Success)}
		if !req.GetSourceOnly() {
			response, err := e.mgetStandardEvalOutputs(ctx, req, frozen)
			if err != nil {
				return nil, err
			}
			if response == nil || len(response.Items) != 1 || response.Items[0] == nil || response.Items[0].GetExptID() != c.ExptID || response.Items[0].GetItemID() != c.ItemID || response.Items[0].GetStatus() != exptdomain.ItemRunState_Success {
				return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
			}
			out = response.Items[0]
			actual, err := port.GetStandardEvalOutputSource(ctx, c.SpaceID, c.ExptID, c.RunID, c.ItemID, &c.RequestID)
			if err != nil {
				return nil, err
			}
			if !service.SameStandardEvalOutputSource(frozen, actual) {
				return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
			}
		}
		out.ExptRunID, out.RequestID, out.SourceTurnRecords = gptr.Of(c.RunID), gptr.Of(c.RequestID), standardEvalOutputSourceRecordsDTO(records)
		items = append(items, out)
	}
	return &expt.MGetExperimentStandardEvalOutputsResponse{Items: items, BaseResp: base.NewBaseResp()}, nil
}

// validateStandardEvalOutputResultSource binds the existing result builder's input to frozen invocation IDs.
// Record SpaceID can belong to an authorized shared resource; the original readers retain that authorization.
func validateStandardEvalOutputResultSource(result *entity.MGetExperimentReportResult, frozen *entity.ExptStandardEvalOutputSource) error {
	mismatch := errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
	if result == nil || len(result.ItemResults) != 1 || result.ItemResults[0] == nil {
		return mismatch
	}
	s := frozen.Snapshot.Source
	c := s.Current
	item := result.ItemResults[0]
	if item.ItemID != c.ItemID || item.SystemInfo == nil || item.SystemInfo.RunState != entity.ItemRunState_Success || len(item.TurnResults) != len(s.CanonicalTurns) {
		return mismatch
	}
	canonical := map[int64]*entity.ExptTurnResult{}
	for _, turn := range s.CanonicalTurns {
		canonical[turn.TurnID] = turn
	}
	seen := map[int64]bool{}
	for _, turn := range item.TurnResults {
		if turn == nil || canonical[turn.TurnID] == nil || seen[turn.TurnID] || len(turn.ExperimentResults) != 1 || turn.ExperimentResults[0] == nil {
			return mismatch
		}
		seen[turn.TurnID] = true
		er := turn.ExperimentResults[0]
		p := er.Payload
		if er.ExperimentID != c.ExptID || p == nil || p.TurnID != turn.TurnID || p.SystemInfo == nil || p.SystemInfo.TurnRunState != entity.TurnRunState_Success {
			return mismatch
		}
		old := canonical[turn.TurnID]
		var target *entity.EvalTargetRecord
		if p.TargetOutput != nil {
			target = p.TargetOutput.EvalTargetRecord
		}
		if old.TargetResultID == 0 {
			if target != nil && target.ID != 0 {
				return mismatch
			}
		} else if target == nil || target.ID != old.TargetResultID || target.ExperimentRunID != c.RunID || target.ItemID != c.ItemID || target.TurnID != turn.TurnID || target.ItemVersionID != 0 && target.ItemVersionID != c.ItemVersionID || target.SpaceID <= 0 {
			return mismatch
		}
		refs := map[int64]*entity.ExptTurnEvaluatorResultRef{}
		for _, ref := range s.CurrentRefs {
			if ref.ExptTurnResultID == old.ID {
				refs[ref.EvaluatorResultID] = ref
			}
		}
		var records map[int64]*entity.EvaluatorRecord
		if p.EvaluatorOutput != nil {
			records = p.EvaluatorOutput.EvaluatorRecords
		}
		if len(records) != len(refs) {
			return mismatch
		}
		seenRecords := map[int64]bool{}
		for _, record := range records {
			if record == nil {
				return mismatch
			}
			ref := refs[record.ID]
			if ref == nil || seenRecords[record.ID] || record.ExperimentID != c.ExptID || record.ExperimentRunID != c.RunID || record.ItemID != c.ItemID || record.TurnID != turn.TurnID || record.SpaceID <= 0 || record.ItemVersionID != 0 && record.ItemVersionID != c.ItemVersionID || record.TargetRecordID != 0 && record.TargetRecordID != old.TargetResultID || record.EvaluatorVersionID != ref.EvaluatorVersionID || record.Alias != ref.Alias || record.InlineKey != ref.InlineKey {
				return mismatch
			}
			if ref.SourceType == int32(entity.EvaluatorRecordSourceTypeInline) && record.SourceType != entity.EvaluatorRecordSourceTypeInline || ref.SourceType != int32(entity.EvaluatorRecordSourceTypeInline) && record.SourceType == entity.EvaluatorRecordSourceTypeInline {
				return mismatch
			}
			seenRecords[record.ID] = true
		}
	}
	return nil
}
