// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/plugin/dbresolver"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	loopjson "github.com/coze-dev/coze-loop/backend/pkg/json"
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

func TestOrdinaryRunLogConversionCapacity(t *testing.T) {
	ids := make([]int64, 4000)
	for i := range ids {
		ids[i] = 9223372036854700000 + int64(i)
	}
	_, err := convert.NewExptRunLogConvertor().DO2PO(&entity.ExptRunLog{ItemIds: []entity.ExptRunLogItems{{ItemIDs: ids}}})
	require.Error(t, err)
	_, ok := kerrors.FromBizStatusError(err)
	require.False(t, ok, "ordinary Create/Save conversion must not produce a manual retry business error")
}

func TestRunLogCapacityMySQLBoundaries(t *testing.T) {
	for _, operation := range []string{"preflight", "requeue", "ordinary_append"} {
		for _, kind := range []string{"capacity", "corrupt"} {
			t.Run(operation+"/"+kind, func(t *testing.T) {
				f := newManualRetryFixture(t)
				req := f.request(entity.EvaluationModeRetryItems)
				raw := []byte(`[{"ItemIDs":[11],"padding":"` + strings.Repeat("x", 65420) + `","future_id":9223372036854775806}]`)
				if kind == "corrupt" {
					raw = []byte(`{`)
				}
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("id=?", f.run).Update("item_ids", raw).Error)
				before := f.snapshot()
				var err error
				switch operation {
				case "preflight":
					plan, callErr := f.port.PreflightTerminalItems(context.Background(), req)
					err = callErr
					require.Nil(t, plan)
				case "requeue":
					receipt, callErr := f.port.RequeueTerminalItems(context.Background(), req)
					err = callErr
					require.Nil(t, receipt)
				case "ordinary_append":
					ids := make([]int64, 20)
					for i := range ids {
						ids[i] = 9223372036854700000 + int64(i)
					}
					err = NewExptRunLogDAO(f.provider).AppendRunItemIDs(context.Background(), f.expt, f.run, f.space, ids)
				}
				require.Error(t, err)
				require.Equal(t, before, f.snapshot(), "capacity/corruption must leave all fixture tables unchanged")
				biz, ok := kerrors.FromBizStatusError(err)
				if kind == "capacity" && operation != "ordinary_append" {
					require.True(t, ok)
					require.Equal(t, int32(601204104), biz.BizStatusCode())
				} else {
					require.False(t, ok)
				}
				if kind == "capacity" {
					require.ErrorIs(t, err, entity.ErrExptRunLogItemsCapacity)
				}
				if kind == "corrupt" {
					require.NotErrorIs(t, err, entity.ErrExptRunLogItemsCapacity)
					if operation == "ordinary_append" {
						var chunks []entity.ExptRunLogItems
						expected := loopjson.Unmarshal(raw, &chunks)
						cause := err
						for errors.Unwrap(cause) != nil {
							cause = errors.Unwrap(cause)
						}
						require.IsType(t, expected, cause)
						require.Equal(t, expected.Error(), cause.Error())
					} else {
						var cause *json.SyntaxError
						require.ErrorAs(t, err, &cause)
					}
				}
			})
		}
	}
}
