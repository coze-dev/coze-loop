package mysql

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

func TestReviewCancelPendingRetryBehindFailedExperiment(t *testing.T) {
	port, conn := pendingCancelDB(t, 3)
	require.NoError(t, conn.Table("experiment").Where("id=2").Update("status", entity.ExptStatus_Failed).Error)
	require.NoError(t, conn.Exec("INSERT INTO expt_run_log(id,space_id,expt_id,expt_run_id,status) VALUES(8,1,2,8,?)", entity.ExptStatus_Failed).Error)
	require.NoError(t, conn.Exec("INSERT INTO expt_item_result(id,space_id,expt_id,expt_run_id,item_id,status) VALUES(7,1,2,8,7,?)", entity.ItemRunState_Success).Error)
	result, err := port.TerminateCurrentRun(context.Background(), &entity.ExptRunTerminationRequest{SpaceID: 1, ExptID: 2, RunID: 3, Finalize: true, Status: entity.ExptStatus_Terminated})
	require.NoError(t, err)
	require.True(t, result.Applied)
	var prior, current, item int32
	require.NoError(t, conn.Table("expt_run_log").Where("id=8").Select("status").Scan(&prior).Error)
	require.NoError(t, conn.Table("expt_run_log").Where("id=3").Select("status").Scan(&current).Error)
	require.NoError(t, conn.Table("expt_item_result").Select("status").Scan(&item).Error)
	require.Equal(t, int32(entity.ExptStatus_Failed), prior)
	require.Equal(t, int32(entity.ExptStatus_Terminated), current)
	require.Equal(t, int32(entity.ItemRunState_Success), item)
}

func TestReviewItemReferenceEnumerationReadsPrimary(t *testing.T) {
	ctx := context.Background()
	provider, err := db.NewDB(sqlite.Open(filepath.Join(t.TempDir(), "primary.db")))
	require.NoError(t, err)
	replicaPath := filepath.Join(t.TempDir(), "replica.db")
	replica, err := gorm.Open(sqlite.Open(replicaPath))
	require.NoError(t, err)
	for conn, count := range map[*gorm.DB]int{provider.NewSession(ctx): 2, replica: 1} {
		require.NoError(t, conn.Exec("CREATE TABLE expt_item_ref(id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, item_id INTEGER, deleted_at DATETIME)").Error)
		for i := 1; i <= count; i++ {
			require.NoError(t, conn.Exec("INSERT INTO expt_item_ref(id,space_id,expt_id,item_id) VALUES(?,1,2,?)", i, 300+i).Error)
		}
		raw, err := conn.DB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = raw.Close() })
	}
	require.NoError(t, provider.NewSession(ctx).Use(dbresolver.Register(dbresolver.Config{Replicas: []gorm.Dialector{sqlite.Open(replicaPath)}})))
	dao := NewExptItemRefDAO(provider)
	stale, _, err := dao.ListByExptID(ctx, 1, 2, 0, 200)
	require.NoError(t, err)
	require.Len(t, stale, 1)
	current, _, err := dao.ListByExptID(contexts.WithCtxWriteDB(ctx), 1, 2, 0, 200)
	require.NoError(t, err)
	require.Len(t, current, 2)
}
