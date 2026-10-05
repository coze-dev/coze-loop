// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"io"
	"slices"
	"strings"
)

type HookRetryItemsTerminalTail struct {
	AcceptedBatches int    `json:"accepted_batch_count"`
	Batch           int    `json:"from_batch"`
	Offset          int    `json:"from_offset"`
	Reason          string `json:"reason"`
}

// Published entries always have manifests after the initial initialization commits.
type HookRetryItemsCursor struct {
	Version     int                         `json:"v"`
	Key         HookRunKey                  `json:"key"`
	Fingerprint string                      `json:"fingerprint"`
	Phase       string                      `json:"phase"`
	Batch       int                         `json:"batch"`
	Offset      int                         `json:"offset"`
	Published   HookPlanDigest              `json:"published"`
	Retries     int                         `json:"retries"`
	Terminal    *HookRetryItemsTerminalTail `json:"terminal_tail,omitempty"`
	Bootstrap   string                      `json:"bootstrap,omitempty"`
}

func decodeRetryItemsJSON(raw string, out any) error {
	if len(raw) == 0 || len(raw) > 65535 {
		return ErrHookStoreCorrupt
	}
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		return ErrHookStoreCorrupt
	}
	return nil
}

func DecodeHookRetryItemsCursor(raw string, key HookRunKey, logs []ExptRunLogItems, count int64, hash string) (HookRetryItemsCursor, error) {
	var c HookRetryItemsCursor
	if decodeRetryItemsJSON(raw, &c) != nil || c.Version != 2 || c.Key != key || c.Retries < 0 || c.Retries > 10 ||
		c.Batch < 0 || c.Batch > len(logs) || c.Offset < 0 || c.Batch == len(logs) && c.Offset != 0 ||
		c.Batch < len(logs) && c.Offset > len(logs[c.Batch].ItemIDs) || c.Published.Count != count || c.Published.Hash != hash {
		return c, ErrHookStoreCorrupt
	}
	if c.Phase != "bootstrap_closed" && (!hookStorageHash(c.Fingerprint) || !hookStorageHash(hash)) {
		return c, ErrHookStoreCorrupt
	}
	switch c.Phase {
	case "tail":
		if c.Terminal != nil || c.Bootstrap != "" {
			return c, ErrHookStoreCorrupt
		}
	case "closed", "bootstrap_closed":
		if c.Terminal == nil || c.Terminal.AcceptedBatches != len(logs) || c.Terminal.Batch != c.Batch || c.Terminal.Offset != c.Offset {
			return c, ErrHookStoreCorrupt
		}
	default:
		return c, ErrHookStoreCorrupt
	}
	return c, nil
}

func (c HookRetryItemsCursor) Encode() (string, error) {
	raw, err := json.Marshal(c)
	if err != nil || len(raw) > 65535 {
		return "", ErrHookStoreCorrupt
	}
	return string(raw), nil
}

func (c HookRetryItemsCursor) Page(logs []ExptRunLogItems) ([]int64, bool) {
	if c.Phase != "tail" || c.Batch >= len(logs) {
		return nil, false
	}
	ids := logs[c.Batch].ItemIDs
	return slices.Clone(ids[c.Offset:min(c.Offset+100, len(ids))]), true
}

func (c HookRetryItemsCursor) Advance(logs []ExptRunLogItems) HookRetryItemsCursor {
	c.Offset = min(c.Offset+100, len(logs[c.Batch].ItemIDs))
	if c.Offset == len(logs[c.Batch].ItemIDs) {
		c.Batch++
		c.Offset = 0
	}
	return c
}

// This is the persisted v1 preparer's shape, not a new selection policy.
type retryItemsBootstrap struct {
	Version      int            `json:"v"`
	Key          HookRunKey     `json:"key"`
	Fingerprint  string         `json:"fingerprint"`
	Phase        string         `json:"phase"`
	Route        string         `json:"route"`
	Selector     string         `json:"selector,omitempty"`
	Batch        int            `json:"batch"`
	BatchDone    bool           `json:"batch_done,omitempty"`
	Retries      int            `json:"retries"`
	Selected     HookPlanDigest `json:"selected"`
	Verified     HookPlanDigest `json:"verified"`
	SourceID     int64          `json:"source_id,omitempty"`
	SourceCount  int64          `json:"source_count,omitempty"`
	SourceHash   string         `json:"source_hash,omitempty"`
	SourceDigest HookPlanDigest `json:"source_digest"`
}

func retryItemsBootstrapCursor(raw string, key HookRunKey, logs []ExptRunLogItems, count int64) (retryItemsBootstrap, error) {
	b := retryItemsBootstrap{Version: 1, Key: key, Phase: "select", Selected: NewHookPlanDigest(), Verified: NewHookPlanDigest(), SourceDigest: NewHookPlanDigest()}
	if raw != "" && decodeRetryItemsJSON(raw, &b) != nil {
		return b, ErrHookStoreCorrupt
	}
	if b.Version != 1 || b.Key != key || b.Batch < 0 || b.Batch > len(logs) || b.Selected.Count != count || b.Retries < 0 || b.Retries > 10 ||
		(b.Phase != "select" && b.Phase != "verify") || (b.Route != "normal" && b.Route != "") || b.SourceID != 0 || b.SourceCount != 0 || b.SourceHash != "" || b.SourceDigest != NewHookPlanDigest() {
		return b, ErrHookStoreCorrupt
	}
	for _, d := range []HookPlanDigest{b.Selected, b.Verified} {
		if _, err := AppendHookPlanDigest(d, nil); err != nil {
			return b, ErrHookStoreCorrupt
		}
	}
	return b, nil
}

func PromoteHookRetryItemsCursor(raw string, key HookRunKey, logs []ExptRunLogItems, count int64, hash string) (string, error) {
	b, err := retryItemsBootstrapCursor(raw, key, logs, count)
	// CompleteExecutionInitialization verifies the final digest; v1 omits its last verification cursor write.
	if err != nil || b.Phase != "verify" || b.Verified.Count > b.Selected.Count || b.Verified.Count == b.Selected.Count && b.Verified.Hash != b.Selected.Hash || b.Selected.Hash != hash || !hookStorageHash(b.Fingerprint) {
		return "", ErrHookStoreCorrupt
	}
	return (HookRetryItemsCursor{Version: 2, Key: key, Fingerprint: b.Fingerprint, Phase: "tail", Batch: b.Batch, Published: b.Selected, Retries: b.Retries}).Encode()
}

func CloseHookRetryItemsCursor(raw string, key HookRunKey, logs []ExptRunLogItems, count int64, hash, reason string) (string, error) {
	c, err := DecodeHookRetryItemsCursor(raw, key, logs, count, hash)
	if err != nil {
		b, err := retryItemsBootstrapCursor(raw, key, logs, count)
		if err != nil {
			return "", err
		}
		c = HookRetryItemsCursor{Version: 2, Key: key, Fingerprint: b.Fingerprint, Phase: "bootstrap_closed", Batch: b.Batch, Published: HookPlanDigest{Count: count, Hash: hash}, Retries: b.Retries, Bootstrap: raw}
		if b.Phase == "select" && !b.BatchDone && b.Selector != "" {
			var pos struct {
				Offset int `json:"o"`
			}
			if json.Unmarshal([]byte(b.Selector), &pos) != nil || pos.Offset < 0 || b.Batch >= len(logs) || pos.Offset > len(logs[b.Batch].ItemIDs) {
				return "", ErrHookStoreCorrupt
			}
			c.Offset = pos.Offset
		}
	} else if c.Phase == "tail" {
		c.Phase = "closed"
	}
	if c.Terminal != nil && c.Terminal.Reason != reason {
		return "", ErrHookStoreConflict
	}
	c.Terminal = &HookRetryItemsTerminalTail{AcceptedBatches: len(logs), Batch: c.Batch, Offset: c.Offset, Reason: reason}
	return c.Encode()
}
