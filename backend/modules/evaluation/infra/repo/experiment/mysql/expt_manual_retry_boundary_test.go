// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/plugin/dbresolver"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestManualRetryReceiptBizBoundary(t *testing.T) {
	receipt := &entity.ManualRetryReceipt{RequestID: "request", PayloadHash: "original"}
	chunks := []entity.ExptRunLogItems{{ManualRetryReceipt: receipt}}
	got, err := findManualRetryReceipt(chunks, "request", "original")
	require.NoError(t, err)
	require.Same(t, receipt, got)
	_, err = findManualRetryReceipt(chunks, "request", "changed")
	biz, ok := kerrors.FromBizStatusError(err)
	require.True(t, ok, "request conflict must survive PacketAdapter")
	require.Equal(t, int32(601204103), biz.BizStatusCode())
}

func TestManualRetryTurnCapacityBizBoundary(t *testing.T) {
	err := validateManualRetryTurnExt([]byte(`{"padding":"` + strings.Repeat("x", 65536) + `"}`))
	biz, ok := kerrors.FromBizStatusError(err)
	require.True(t, ok, "turn ext capacity must survive PacketAdapter")
	require.Equal(t, int32(601204104), biz.BizStatusCode())
}

func TestManualRetryReceiptDBErrorRetainsCause(t *testing.T) {
	conn, source, _ := newRunLogResolverTestDB(t)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(conn.Clauses(dbresolver.Write))
	cause := errors.New("database unavailable")
	source.ExpectQuery("SELECT .* FROM `expt_run_log`").WillReturnError(cause)
	_, err := NewExptManualRetryDAO(provider, nil).FindReceipt(context.Background(), 9, 7, 8, "request", "hash")
	require.ErrorIs(t, err, cause)
	_, ok := kerrors.FromBizStatusError(err)
	require.False(t, ok)
	require.NoError(t, source.ExpectationsWereMet())
}
