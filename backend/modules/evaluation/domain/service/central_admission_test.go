// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

func newTestGate(t *testing.T, mode string, spaces, expts []int64) (*CentralGate, *central.MemoryStore, *central.Admission, *componentmocks.MockIConfiger) {
	t.Helper()
	ctrl := gomock.NewController(t)
	configer := componentmocks.NewMockIConfiger(ctrl)
	conf := &entity.ExptExecConf{
		CentralSchedulingMode:       mode,
		CentralSchedulingSpaceIDs:   spaces,
		CentralSchedulingExptIDs:    expts,
		CentralResourceItemSlot:     10,
		CentralResourceTargetSlot:   10,
		CentralResourceSandboxSlot:  10,
		CentralReservationTTLSecond: 60,
	}
	configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(conf).AnyTimes()

	clock := fixedClock{t: time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)}
	store := central.NewMemoryStore(clock)
	adm := &central.Admission{
		Store: store,
		Clock: clock,
		LimitsFn: func(_ central.Ctx, _ int64) (central.Limits, error) {
			return central.Limits{
				Capacity:       CentralCapacity(conf.GetCentralCapacity()),
				ReservationTTL: conf.GetCentralReservationTTL(),
			}, nil
		},
		ModeFn: func(ctx central.Ctx, key central.RunKey) central.Mode {
			// The gate's config-driven allow-list is exercised separately; here we
			// mirror the effective mode the gate resolved.
			spaceConf := configer.GetExptExecConf(ctx, key.SpaceID)
			m := central.ParseMode(spaceConf.GetCentralSchedulingMode())
			if m == central.ModeEnforce && !spaceConf.CentralEnforced(key.SpaceID, key.ExptID) {
				return central.ModeLegacy
			}
			return m
		},
		ScopeFn: func(_ central.Ctx, spaceID int64) central.Scope {
			return central.Scope{Env: "ppe_gate_test", Region: "cn", SpaceID: spaceID}
		},
	}
	return NewCentralGate(adm, configer).(*CentralGate), store, adm, configer
}

func testEvent() *entity.ExptScheduleEvent {
	return &entity.ExptScheduleEvent{
		SpaceID: 11, ExptID: 22, ExptRunID: 33,
		ExptRunMode: entity.EvaluationModeSubmit,
		CreatedAt:   time.Now().Unix(),
		Session:     &entity.Session{UserID: "u1"},
	}
}

func TestCentralGateLegacyIsNoop(t *testing.T) {
	gate, store, _, _ := newTestGate(t, "legacy", nil, nil)
	ctx := context.Background()
	event := testEvent()
	if got := gate.Mode(ctx, event); got != central.ModeLegacy {
		t.Fatalf("mode=%v want legacy", got)
	}
	granted, err := gate.AcquireItems(ctx, event, []int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(granted) != 0 {
		t.Fatalf("legacy gate must not grant, got %v", granted)
	}
	reservations, _ := store.ListReservations(ctx, central.Scope{Env: "ppe_gate_test", Region: "cn"})
	if len(reservations) != 0 {
		t.Fatalf("legacy gate must not reserve, got %d", len(reservations))
	}
}

func TestCentralGateEnforceRequiresAllowList(t *testing.T) {
	gate, store, _, _ := newTestGate(t, "enforce", nil, nil)
	ctx := context.Background()
	event := testEvent()
	if got := gate.Mode(ctx, event); got != central.ModeLegacy {
		t.Fatalf("enforce without allow-list must fall back to legacy, got %v", got)
	}
	gate2, store2, _, _ := newTestGate(t, "enforce", []int64{event.SpaceID}, nil)
	if got := gate2.Mode(ctx, event); got != central.ModeEnforce {
		t.Fatalf("allow-listed space must be enforce, got %v", got)
	}
	_ = store
	_ = store2
}

func TestCentralGateEnforceReservesAndReleases(t *testing.T) {
	gate, store, _, _ := newTestGate(t, "enforce", []int64{11}, nil)
	ctx := context.Background()
	event := testEvent()
	granted, err := gate.AcquireItems(ctx, event, []int64{101, 102})
	if err != nil {
		t.Fatal(err)
	}
	if len(granted) != 2 {
		t.Fatalf("granted=%v", granted)
	}
	scope := central.Scope{Env: "ppe_gate_test", Region: "cn", SpaceID: 11}
	reservations, _ := store.ListReservations(ctx, scope)
	if len(reservations) != 2 {
		t.Fatalf("reservations=%d", len(reservations))
	}
	gate.ReleaseItems(ctx, event, granted)
	reservations, _ = store.ListReservations(ctx, scope)
	if len(reservations) != 0 {
		t.Fatalf("release left %d reservations", len(reservations))
	}
}

func TestCentralGateShadowDoesNotMutate(t *testing.T) {
	gate, store, _, _ := newTestGate(t, "shadow", nil, nil)
	ctx := context.Background()
	event := testEvent()
	granted, err := gate.AcquireItems(ctx, event, []int64{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(granted) != 3 {
		t.Fatalf("shadow must keep dispatch unchanged, got %v", granted)
	}
	reservations, _ := store.ListReservations(ctx, central.Scope{Env: "ppe_gate_test", Region: "cn", SpaceID: 11})
	if len(reservations) != 0 {
		t.Fatalf("shadow created %d reservations", len(reservations))
	}
}

func TestFilterItemsByID(t *testing.T) {
	items := []*entity.ExptEvalItem{{ItemID: 1}, {ItemID: 2}, {ItemID: 3}, nil}
	got := filterItemsByID(items, []int64{1, 3})
	if len(got) != 2 || got[0].ItemID != 1 || got[1].ItemID != 3 {
		t.Fatalf("filtered=%v", got)
	}
	if got := filterItemsByID(items, nil); got != nil {
		t.Fatalf("empty allow-list must return nil, got %v", got)
	}
}
