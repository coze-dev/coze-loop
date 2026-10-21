// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type PreparedRetryItem struct {
	PreviousRunID     int64
	PreviousRequestID string
	ItemVersionID     int64
	SourceItem        *ExptItemResult
	CurrentItemLog    *ExptItemResultRunLog
	FrozenItemRef     *ExptItemRef
	SingleSetSource   *PreparedRetrySingleSetSource
	SourceTurnLogs    []*ExptTurnResultRunLog
	Turns             []*PreparedRetryTurn
}

// PreparedRetrySingleSetSource freezes the existing experiment binding for the
// single-set path, whose start does not materialize expt_item_ref rows.
// It is preparation metadata only and is never persisted as a synthetic ref.
type PreparedRetrySingleSetSource struct {
	SourceType       ExptEvalSetSourceType
	EvalSetID        int64
	EvalSetVersionID int64
	EvalSetSpaceID   int64
	TargetID         int64
	TargetVersionID  int64
	TargetSpaceID    int64
	TargetType       EvalTargetType
}

type PreparedRetryTurn struct {
	SourceTurn       *ExptTurnResult
	CurrentRunLog    *ExptTurnResultRunLog
	SourceRefs       []*ExptTurnEvaluatorResultRef
	TargetResultID   int64
	EvaluatorResults *EvaluatorResults
	KeepRefIDs       []int64
	PreserveSuccess  bool
}

type ManualRetryPreparation struct {
	Items         []*PreparedRetryItem
	RejectedItems []*ManualRetryRejectedItem
	SnapshotAtMS  int64
}

type RequeueTerminalItemsRequest struct {
	SpaceID       int64
	ExptID        int64
	RunID         int64
	RequestID     string
	PayloadHash   string
	Operator      string
	Mode          ExptRunMode
	SnapshotAtMS  int64
	Items         []*PreparedRetryItem
	RejectedItems []*ManualRetryRejectedItem
}

type RetryInCurrentRunRequest struct {
	SpaceID       int64
	ExptID        int64
	ExpectedRunID int64
	RequestID     string
	Mode          ExptRunMode
	ItemIDs       []int64
	Ext           map[string]string
	Session       *Session
}
