// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestHookWorkerClockUsesPrimaryStorageWallTime(t *testing.T) {
	p, m := gateMock(t)
	clock, err := NewHookWorkerClock(p)
	require.NoError(t, err)
	at := time.Date(2026, 9, 23, 8, 0, 0, 123000000, time.FixedZone("storage-zone", 8*3600))
	m.ExpectQuery(`^SELECT CURRENT_TIMESTAMP\(3\)$`).WillReturnRows(sqlmock.NewRows([]string{"now"}).AddRow(at))
	got, err := clock.Now(context.Background())
	require.NoError(t, err)
	require.Equal(t, at, got)
	require.Equal(t, "2026-09-23 08:00:00.123", hookScanSQLTime(got))
	m.ExpectQuery(`^SELECT CURRENT_TIMESTAMP\(3\)$`).WillReturnError(errors.New("raw database secret"))
	_, err = clock.Now(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "raw")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = clock.Now(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestHookWorkerClockRejectsMissingProvider(t *testing.T) {
	_, err := NewHookWorkerClock(nil)
	require.Error(t, err)
	var typedNil *gateMasterProvider
	_, err = NewHookWorkerClock(typedNil)
	require.Error(t, err)
}
