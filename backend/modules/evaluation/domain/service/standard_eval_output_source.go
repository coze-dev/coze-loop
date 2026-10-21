// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"reflect"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// ExptStandardEvalOutputSourceService is optional for legacy implementations. Explicit source requests fail closed.
type ExptStandardEvalOutputSourceService interface {
	GetStandardEvalOutputSource(context.Context, int64, int64, int64, int64, ...*string) (*entity.ExptStandardEvalOutputSource, error)
}

var _ ExptStandardEvalOutputSourceService = (*ExptResultServiceImpl)(nil)

func (e *ExptResultServiceImpl) GetStandardEvalOutputSource(ctx context.Context, space, expt, run, item int64, expectedRequestID ...*string) (*entity.ExptStandardEvalOutputSource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	port, ok := e.resultPublishRepo.(repo.IExptStandardEvalOutputSourceRepo)
	if !ok {
		return nil, errorx.NewByCode(errno.StandardEvalOutputSourceNotReadyCode)
	}
	snapshot, err := port.GetStandardEvalOutputSource(ctx, space, expt, run, item)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
	}
	if snapshot.Source == nil || snapshot.Item == nil || snapshot.ItemLog == nil {
		return nil, errorx.NewByCode(errno.StandardEvalOutputSourceNotReadyCode)
	}
	c := snapshot.Source.Current
	if c.SpaceID != space || c.ExptID != expt || c.ItemID != item || c.RunID <= 0 || run > 0 && c.RunID != run {
		return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
	}
	if len(expectedRequestID) > 0 && expectedRequestID[0] != nil && c.RequestID != *expectedRequestID[0] {
		return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
	}
	if c.Status != entity.ItemRunState_Success || c.ResultState != entity.ExptItemResultStateResulted || snapshot.Item.Status != entity.ItemRunState_Success || snapshot.ItemLog.Status != int32(entity.ItemRunState_Success) || snapshot.ItemLog.ResultState != int32(entity.ExptItemResultStateResulted) {
		return nil, errorx.NewByCode(errno.StandardEvalOutputSourceNotReadyCode)
	}
	frozen := &entity.ExptStandardEvalOutputSource{Snapshot: snapshot}
	if _, err := StandardEvalOutputSourceRecords(frozen); err != nil {
		return nil, err
	}
	return frozen, nil
}

// StandardEvalOutputSourceRecords shares the MQ source validator while requiring persisted canonical Resulted data.
func StandardEvalOutputSourceRecords(frozen *entity.ExptStandardEvalOutputSource) ([]component.ItemCompleteTurnSource, error) {
	if frozen == nil || frozen.Snapshot == nil || frozen.Snapshot.Source == nil {
		return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
	}
	s := frozen.Snapshot
	c := s.Source.Current
	if c.SpaceID <= 0 || c.ExptID <= 0 || c.RunID <= 0 || c.ItemID <= 0 || c.ItemVersionID < 0 || s.Item == nil || s.ItemLog == nil || s.Item.Status != entity.ItemRunState_Success || c.Status != entity.ItemRunState_Success || c.ResultState != entity.ExptItemResultStateResulted {
		return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
	}
	for _, turn := range s.Source.CanonicalTurns {
		if turn == nil {
			return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
		}
		log := s.Source.Turns[turn.TurnID]
		if log == nil || turn.Status != int32(log.Status) || turn.TargetResultID != log.TargetResultID || turn.TraceID != log.TraceID || turn.LogID != log.LogID || turn.ErrMsg != log.ErrMsg {
			return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode)
		}
	}
	records, err := buildItemCompleteSourceRecords(&entity.ExptItemResultPublication{Source: s, Turns: s.Source.CanonicalTurns, Refs: s.Source.CurrentRefs}, entity.ExptItemResultStateResulted)
	if err != nil {
		return nil, errorx.NewByCode(errno.StandardEvalOutputSourceMismatchCode, errorx.WithExtraMsg(err.Error()))
	}
	return records, nil
}

// SameStandardEvalOutputSource compares invocation and physical canonical reference identities, not record contents.
func SameStandardEvalOutputSource(a, b *entity.ExptStandardEvalOutputSource) bool {
	if a == nil || b == nil || a.Snapshot == nil || b.Snapshot == nil || a.Snapshot.Source == nil || b.Snapshot.Source == nil {
		return false
	}
	x, y := a.Snapshot.Source.Current, b.Snapshot.Source.Current
	if a.Snapshot.Item == nil || b.Snapshot.Item == nil || a.Snapshot.ItemLog == nil || b.Snapshot.ItemLog == nil || x.SpaceID != y.SpaceID || x.ExptID != y.ExptID || x.RunID != y.RunID || x.ItemID != y.ItemID || x.ItemVersionID != y.ItemVersionID || x.RequestID != y.RequestID || x.Mode != y.Mode || a.Snapshot.Item.ID != b.Snapshot.Item.ID || a.Snapshot.ItemLog.ID != b.Snapshot.ItemLog.ID {
		return false
	}
	ar, ae := StandardEvalOutputSourceRecords(a)
	br, be := StandardEvalOutputSourceRecords(b)
	if ae != nil || be != nil || !reflect.DeepEqual(ar, br) || len(a.Snapshot.Source.CanonicalTurns) != len(b.Snapshot.Source.CanonicalTurns) || !reflect.DeepEqual(a.Snapshot.Source.CurrentRefs, b.Snapshot.Source.CurrentRefs) || !reflect.DeepEqual(a.Snapshot.ItemRef, b.Snapshot.ItemRef) || !reflect.DeepEqual(a.Snapshot.ItemConfigJSON, b.Snapshot.ItemConfigJSON) {
		return false
	}
	for i, turn := range a.Snapshot.Source.CanonicalTurns {
		other := b.Snapshot.Source.CanonicalTurns[i]
		if turn.ID != other.ID || turn.TurnID != other.TurnID || a.Snapshot.Source.Turns[turn.TurnID].ID != b.Snapshot.Source.Turns[other.TurnID].ID {
			return false
		}
	}
	return true
}
