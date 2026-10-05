// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"unicode/utf8"
)

var ErrHookPlanStorage = errors.New("hook plan storage unavailable")

type HookPlanReadInput struct {
	Key            HookRunKey
	ExecutionScope string
	StartOrdinal   int64
	Limit          int32
}

type HookPlanReadPage struct {
	Items       []HookPlanItem
	NextOrdinal int64
	Count       int64
	Hash        string
	Ready       bool
	RunVersion  int64
	HasMore     bool
}

type HookAdvancePlanInput struct {
	HookStoreGuard
	ExecutionScope     string
	ExpectedCount      int64
	Cursor, NextCursor string
}

type HookPlanLookupInput struct {
	Key            HookRunKey
	ExecutionScope string
	ItemIDs        []int64
}

type HookPlanLookupResult struct {
	Items      []HookPlanItem
	RunVersion int64
	Count      int64
	Ready      bool
}

func (in HookPlanLookupInput) Validate() error {
	if !validHookRunKey(in.Key) || !hookStorageASCII(in.ExecutionScope, 128) || len(in.ItemIDs) < 1 || len(in.ItemIDs) > 100 {
		return invalidParam("invalid hook plan lookup boundary")
	}
	seen := make(map[int64]bool, len(in.ItemIDs))
	for _, id := range in.ItemIDs {
		if id <= 0 || seen[id] {
			return invalidParam("invalid or duplicate hook plan lookup item")
		}
		seen[id] = true
	}
	return nil
}

func (in HookPlanReadInput) Validate() error {
	if !validHookRunKey(in.Key) || !hookStorageASCII(in.ExecutionScope, 128) || in.StartOrdinal < 0 || in.Limit < 1 || in.Limit > 100 {
		return invalidParam("invalid hook plan read boundary")
	}
	return nil
}

func (in HookAdvancePlanInput) Validate() error {
	if err := in.HookStoreGuard.Validate(); err != nil {
		return err
	}
	if !hookStorageASCII(in.ExecutionScope, 128) || in.ExpectedCount < 0 || in.Cursor == in.NextCursor || len(in.Cursor) > 65535 || len(in.NextCursor) > 65535 || !utf8.ValidString(in.Cursor) || !utf8.ValidString(in.NextCursor) {
		return invalidParam("invalid hook plan cursor advance")
	}
	return nil
}
