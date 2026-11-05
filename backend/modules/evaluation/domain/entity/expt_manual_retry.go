// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

const ManualRetryExtKey = "manual_retry"

type ManualRetryContext struct {
	RunID        int64       `json:"run_id"`
	RequestID    string      `json:"request_id"`
	Mode         ExptRunMode `json:"mode"`
	Operator     string      `json:"operator"`
	AcceptedAtMS int64       `json:"accepted_at_ms"`
}

type ManualRetryReceipt struct {
	RunID         int64                      `json:"run_id,omitempty"`
	RequestID     string                     `json:"request_id"`
	PayloadHash   string                     `json:"payload_hash"`
	AcceptedCount int32                      `json:"accepted_count"`
	RejectedItems []*ManualRetryRejectedItem `json:"rejected_items"`
	SnapshotAtMS  int64                      `json:"snapshot_at_ms"`
	unknownFields map[string]json.RawMessage
}

type ManualRetryRejectedItem struct {
	ItemID        int64  `json:"item_id"`
	Reason        string `json:"reason"`
	unknownFields map[string]json.RawMessage
}

// ParseExptItemResultExt validates both JSON layers before any retry merge.
func ParseExptItemResultExt(raw []byte) (map[string]string, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("parse experiment item ext: %w", err)
	}
	ext := make(map[string]string, len(values))
	for key, value := range values {
		var str *string
		if err := json.Unmarshal(value, &str); err != nil || str == nil {
			return nil, fmt.Errorf("experiment item ext %q must be a string", key)
		}
		ext[key] = *str
	}
	if _, err := ParseManualRetryContext(ext); err != nil {
		return nil, err
	}
	return ext, nil
}

func ParseManualRetryContext(ext map[string]string) (*ManualRetryContext, error) {
	raw, exists := ext[ManualRetryExtKey]
	if !exists {
		return nil, nil
	}
	var retry *ManualRetryContext
	if err := json.Unmarshal([]byte(raw), &retry); err != nil {
		return nil, fmt.Errorf("parse manual retry context: %w", err)
	}
	if retry == nil || retry.RunID <= 0 || retry.RequestID == "" ||
		(retry.Mode != EvaluationModeFailRetry && retry.Mode != EvaluationModeRetryItems) {
		return nil, fmt.Errorf("invalid manual retry context")
	}
	return retry, nil
}

func MergeManualRetryContext(raw []byte, retry *ManualRetryContext) ([]byte, error) {
	ext, err := ParseExptItemResultExt(raw)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(retry)
	if err != nil {
		return nil, err
	}
	if _, err := ParseManualRetryContext(map[string]string{ManualRetryExtKey: string(encoded)}); err != nil {
		return nil, err
	}
	if ext == nil {
		ext = make(map[string]string)
	}
	ext[ManualRetryExtKey] = string(encoded)
	encoded, err = json.Marshal(ext)
	if err == nil && len(encoded) > 65535 {
		return nil, errorx.NewByCode(errno.ManualRetryCapacityCode, errorx.WithExtraMsg("manual retry ext exceeds blob capacity"))
	}
	return encoded, err
}

// HasManualRetryContext 返回 ext 是否携带 manual retry 请求上下文。
//
// 它是「新 manual retry 语义」的作用域标记：只有用户显式发起的同 run 重试才带这个键
// （RequeueTerminalItems 合并进 expt_item_result.ext，下一次换 run 时由
// newRunItemResultFields 清除）。执行侧据此决定是否下发多轮 HistoryMessages，
// 普通 submit / fail-retry 不受影响。
func HasManualRetryContext(ext map[string]string) bool {
	if len(ext) == 0 {
		return false
	}
	_, exists := ext[ManualRetryExtKey]
	return exists
}

func WithoutManualRetryExt(ext map[string]string) map[string]string {
	result := maps.Clone(ext)
	delete(result, ManualRetryExtKey)
	return result
}

// ErrExptRunLogItemsCapacity identifies BLOB overflow without assigning a business status.
var ErrExptRunLogItemsCapacity = errors.New("experiment run item metadata exceeds blob capacity")

func EncodeExptRunLogItems(items []ExptRunLogItems) ([]byte, error) {
	encoded, err := json.Marshal(items)
	if err == nil && len(encoded) > 65535 {
		return nil, ErrExptRunLogItemsCapacity
	}
	return encoded, err
}

// Raw unknown fields retain future metadata without rounding large numeric IDs.
func decodeRunMetadata(raw []byte, value any, known ...string) (map[string]json.RawMessage, error) {
	if err := json.Unmarshal(raw, value); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for _, key := range known {
		delete(fields, key)
	}
	return fields, nil
}

func encodeRunMetadata(value any, unknown map[string]json.RawMessage) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil || len(unknown) == 0 {
		return raw, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for key, value := range unknown {
		if _, exists := fields[key]; !exists {
			fields[key] = value
		}
	}
	return json.Marshal(fields)
}

func (i *ExptRunLogItems) UnmarshalJSON(raw []byte) error {
	type plain ExptRunLogItems
	var value plain
	unknown, err := decodeRunMetadata(raw, &value, "ItemIDs", "CreateAt", "manual_retry_receipt")
	if err != nil {
		return err
	}
	*i = ExptRunLogItems(value)
	i.unknownFields = unknown
	return nil
}

func (i ExptRunLogItems) MarshalJSON() ([]byte, error) {
	type plain ExptRunLogItems
	return encodeRunMetadata(plain(i), i.unknownFields)
}

func (r *ManualRetryReceipt) UnmarshalJSON(raw []byte) error {
	type plain ManualRetryReceipt
	var value plain
	unknown, err := decodeRunMetadata(raw, &value, "run_id", "request_id", "payload_hash", "accepted_count", "rejected_items", "snapshot_at_ms")
	if err != nil {
		return err
	}
	*r = ManualRetryReceipt(value)
	r.unknownFields = unknown
	return nil
}

func (r ManualRetryReceipt) MarshalJSON() ([]byte, error) {
	type plain ManualRetryReceipt
	return encodeRunMetadata(plain(r), r.unknownFields)
}

func (r *ManualRetryRejectedItem) UnmarshalJSON(raw []byte) error {
	type plain ManualRetryRejectedItem
	var value plain
	unknown, err := decodeRunMetadata(raw, &value, "item_id", "reason")
	if err != nil {
		return err
	}
	*r = ManualRetryRejectedItem(value)
	r.unknownFields = unknown
	return nil
}

func (r ManualRetryRejectedItem) MarshalJSON() ([]byte, error) {
	type plain ManualRetryRejectedItem
	return encodeRunMetadata(plain(r), r.unknownFields)
}
