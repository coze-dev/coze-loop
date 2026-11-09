// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "strconv"

const (
	targetRecordSourceSpaceKey    = "target_source_space_id"
	targetRecordExecutionSpaceKey = "execution_space_id"
)

func ResolveTargetExecutionSpace(sourceSpaceID, experimentSpaceID int64) int64 {
	if experimentSpaceID > 0 {
		return experimentSpaceID
	}
	return sourceSpaceID
}

func (record *EvalTargetRecord) BindExecutionSpace(sourceSpaceID, experimentSpaceID int64) {
	ext := make(map[string]string, len(record.Ext))
	for key, value := range record.Ext {
		if key != targetRecordSourceSpaceKey && key != targetRecordExecutionSpaceKey {
			ext[key] = value
		}
	}
	record.SpaceID = ResolveTargetExecutionSpace(sourceSpaceID, experimentSpaceID)
	if experimentSpaceID > 0 {
		ext[targetRecordSourceSpaceKey] = strconv.FormatInt(sourceSpaceID, 10)
		ext[targetRecordExecutionSpaceKey] = strconv.FormatInt(experimentSpaceID, 10)
	}
	record.Ext = ext
}

func (record *EvalTargetRecord) TargetSourceSpaceID() int64 {
	if record == nil {
		return 0
	}
	if source, ok := record.Ext[targetRecordSourceSpaceKey]; ok {
		spaceID, err := strconv.ParseInt(source, 10, 64)
		if err != nil || spaceID <= 0 {
			return 0
		}
		return spaceID
	}
	return record.SpaceID
}

func (record *EvalTargetRecord) ExperimentOwned() bool {
	if record == nil || record.SpaceID <= 0 {
		return false
	}
	return record.Ext[targetRecordExecutionSpaceKey] == strconv.FormatInt(record.SpaceID, 10)
}
