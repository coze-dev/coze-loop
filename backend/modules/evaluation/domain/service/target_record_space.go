// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ExperimentTargetRecordSpaces keeps legacy source-owned records reachable. Callers
// must obtain record IDs and the frozen source workspace from an authorized experiment.
func ExperimentTargetRecordSpaces(spaceID, sourceSpaceID int64) []int64 {
	spaces := []int64{spaceID}
	if sourceSpaceID > 0 && sourceSpaceID != spaceID {
		spaces = append(spaces, sourceSpaceID)
	}
	return spaces
}

func GetExperimentTargetRecord(ctx context.Context, svc IEvalTargetService, spaceID, sourceSpaceID, recordID int64) (*entity.EvalTargetRecord, error) {
	for _, recordSpaceID := range ExperimentTargetRecordSpaces(spaceID, sourceSpaceID) {
		record, err := svc.GetRecordByID(ctx, recordSpaceID, recordID)
		if err != nil || record != nil {
			return record, err
		}
	}
	return nil, nil
}

func BatchGetExperimentTargetRecords(ctx context.Context, svc IEvalTargetService, spaceID, sourceSpaceID int64, recordIDs []int64) ([]*entity.EvalTargetRecord, error) {
	missing := recordIDs
	var records []*entity.EvalTargetRecord
	for _, recordSpaceID := range ExperimentTargetRecordSpaces(spaceID, sourceSpaceID) {
		if len(missing) == 0 {
			break
		}
		batch, err := svc.BatchGetRecordByIDs(ctx, recordSpaceID, missing)
		if err != nil {
			return nil, err
		}
		records = append(records, batch...)
		found := make(map[int64]struct{}, len(batch))
		for _, record := range batch {
			if record != nil {
				found[record.ID] = struct{}{}
			}
		}
		next := make([]int64, 0, len(missing))
		for _, id := range missing {
			if _, ok := found[id]; !ok {
				next = append(next, id)
			}
		}
		missing = next
	}
	return records, nil
}

// The record is already scoped to an authorized execution. Its definition can
// belong to a shared source workspace; never use that workspace to read records.
func (e *EvalTargetServiceImpl) recordTargetVersion(ctx context.Context, record *entity.EvalTargetRecord) (*entity.EvalTarget, error) {
	version, err := e.evalTargetRepo.GetEvalTargetVersion(ctx, record.SpaceID, record.TargetVersionID)
	if err == nil {
		return version, nil
	}
	if status, ok := errorx.FromStatusError(err); !ok || status.Code() != errno.ResourceNotFoundCode {
		return nil, err
	}
	if record.TargetID <= 0 {
		return nil, err
	}
	target, lookupErr := e.evalTargetRepo.GetEvalTarget(ctx, record.TargetID)
	if lookupErr != nil {
		return nil, lookupErr
	}
	if target == nil || target.SpaceID <= 0 || target.SpaceID == record.SpaceID {
		return nil, err
	}
	version, err = e.evalTargetRepo.GetEvalTargetVersion(ctx, target.SpaceID, record.TargetVersionID)
	if err != nil {
		return nil, err
	}
	if version == nil || version.ID != record.TargetID {
		return nil, fmt.Errorf("target record definition mismatch: record_id=%d", record.ID)
	}
	return version, nil
}

func (e *EvalTargetServiceImpl) recordTargetVersions(ctx context.Context, spaceID int64, versionIDs []int64, records []*entity.EvalTargetRecord) ([]*entity.EvalTarget, error) {
	versions, err := e.evalTargetRepo.BatchGetEvalTargetVersion(ctx, spaceID, versionIDs)
	if err != nil {
		return nil, err
	}
	found := make(map[int64]bool, len(versions))
	for _, v := range versions {
		if v != nil && v.EvalTargetVersion != nil {
			found[v.EvalTargetVersion.ID] = true
		}
	}
	for _, record := range records {
		if record == nil || record.TargetID <= 0 || record.TargetVersionID <= 0 || found[record.TargetVersionID] {
			continue
		}
		v, err := e.recordTargetVersion(ctx, record)
		if err != nil {
			return nil, err
		}
		if v != nil {
			versions = append(versions, v)
		}
		found[record.TargetVersionID] = true
	}
	return versions, nil
}

func terminateExperimentTargetRecords(ctx context.Context, svc IEvalTargetService, spaceID, sourceSpaceID int64, ids []int64, code int32, message string, zombie bool, wait ...bool) error {
	var result error
	for _, recordSpaceID := range ExperimentTargetRecordSpaces(spaceID, sourceSpaceID) {
		result = errors.Join(result, svc.TerminateAsyncRecordsAndDestroySandbox(ctx, recordSpaceID, ids, code, message, zombie, wait...))
	}
	return result
}

func checkExperimentSandboxTerminated(ctx context.Context, svc IEvalTargetService, spaceID, sourceSpaceID int64, ids []int64) ([]int64, map[int64]string) {
	var terminated []int64
	statuses := make(map[int64]string)
	for _, recordSpaceID := range ExperimentTargetRecordSpaces(spaceID, sourceSpaceID) {
		found, status := svc.CheckSandboxTerminated(ctx, recordSpaceID, ids)
		terminated = append(terminated, found...)
		for id, value := range status {
			statuses[id] = value
		}
	}
	return terminated, statuses
}
