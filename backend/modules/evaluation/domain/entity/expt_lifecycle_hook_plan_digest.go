// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math"
)

// HookPlanDigest resumes the v1 row chain without keeping previous pages in memory.
type HookPlanDigest struct {
	Count int64
	Hash  string
}

func NewHookPlanDigest() HookPlanDigest {
	hash := sha256.Sum256([]byte("hook-plan-row-chain-v1"))
	return HookPlanDigest{Hash: hex.EncodeToString(hash[:])}
}

func AppendHookPlanDigest(previous HookPlanDigest, items []HookPlanItem) (HookPlanDigest, error) {
	invalid := errors.New("invalid hook plan digest")
	chain, err := hex.DecodeString(previous.Hash)
	if err != nil || len(chain) != sha256.Size || hex.EncodeToString(chain) != previous.Hash || previous.Count < 0 || len(items) > 100 || previous.Count > math.MaxInt64-int64(len(items)) {
		return HookPlanDigest{}, invalid
	}
	if previous.Count == 0 && previous != NewHookPlanDigest() {
		return HookPlanDigest{}, invalid
	}
	count := previous.Count
	for _, item := range items {
		if item.SourceSpaceID <= 0 || item.EvalSetID <= 0 || item.ItemID <= 0 || item.EvalSetVersionID < 0 || item.ItemVersionID < 0 {
			return HookPlanDigest{}, invalid
		}
		var row [80]byte
		copy(row[:32], chain)
		// Fixed-width big-endian ordinal and source metadata exclude the database PK.
		for i, value := range [...]int64{count, item.SourceSpaceID, item.EvalSetID, item.EvalSetVersionID, item.ItemID, item.ItemVersionID} {
			binary.BigEndian.PutUint64(row[32+i*8:], uint64(value))
		}
		next := sha256.Sum256(row[:])
		chain = next[:]
		count++
	}
	return HookPlanDigest{Count: count, Hash: hex.EncodeToString(chain)}, nil
}
