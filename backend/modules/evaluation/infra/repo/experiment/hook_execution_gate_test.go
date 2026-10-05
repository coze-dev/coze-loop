package experiment

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHookExecutionMySQLAdmissionRequiresOwnMarker(t *testing.T) {
	for _, atomic := range []bool{false, true} {
		t.Run(map[bool]string{false: "read_gate", true: "atomic_admit"}[atomic], func(t *testing.T) {
			f := newExecutionFixture(t, 1)
			ctx := context.Background()
			require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).Update("status", int64(entity.ExptStatus_Processing)).Error)
			pre, err := NewHookGateRepo(f.p, func(context.Context) (string, error) { return "local", nil }).CanDispatch(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateReady, pre.Gate)
			run, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			if atomic {
				_, err = f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: f.manifests[0].Frozen.ItemID})
				require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
			} else {
				gate, err := NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil }).CanDispatch(ctx, f.key)
				require.NoError(t, err)
				require.Equal(t, entity.HookGateWaiting, gate.Gate)
			}
			page, err := f.init.WriteExecutionInitializationPage(ctx, f.writeInput(run.Version))
			require.NoError(t, err)
			done, err := f.init.CompleteExecutionInitialization(ctx, f.completeInput(page.RunVersion))
			require.NoError(t, err)
			gate, err := NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil }).CanDispatch(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateReady, gate.Gate)
			admitted, err := f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: done.RunVersion}, ItemID: f.manifests[0].Frozen.ItemID})
			require.NoError(t, err)
			require.True(t, admitted.Admitted)
		})
	}
}
