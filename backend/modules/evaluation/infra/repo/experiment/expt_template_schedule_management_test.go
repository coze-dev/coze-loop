// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"os"
	"sync"
	"testing"
)

func NewTemplateScheduleChainTestDependencies(t *testing.T) (db.Provider, repo.IHookConfigRepo, repo.IExptTemplateScheduleStore, repo.IExptTemplateScheduleBindingRepo) {
	t.Helper()
	p := bindingMySQLProvider(t)
	codec := configTestCodec(t)
	return p, NewHookConfigRepo(p, codec), NewExptTemplateScheduleStore(p, codec), NewExptTemplateScheduleBindingRepo(p)
}

func TestTemplateScheduleManagementMySQL(t *testing.T) {
	require.NotEmpty(t, os.Getenv("HOOK_MYSQL_TEST_DSN"))
	p := bindingMySQLProvider(t)
	ctx := context.Background()
	s := p.NewSession(ctx, db.WithMaster())
	key := bindingMySQLSeed(t, p, 7378265404009201)
	require.NoError(t, s.Where("id=? AND space_id=?", key.TemplateID, key.SpaceID).Delete(&model.ExptTemplate{}).Error)
	// The existing fixture cleanup owns this ID; remove its soft-deleted seed before create.
	require.NoError(t, s.Unscoped().Where("id=? AND space_id=?", key.TemplateID, key.SpaceID).Delete(&model.ExptTemplate{}).Error)
	store := NewExptTemplateScheduleStore(p, configTestCodec(t))
	bindings := NewExptTemplateScheduleBindingRepo(p)
	input := repo.ExptTemplateScheduleWrite{Key: key, Create: true, Template: &entity.ExptTemplate{Meta: &entity.ExptTemplateMeta{ID: key.TemplateID, WorkspaceID: key.SpaceID, Name: "managed-original"}, BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of(bindingMySQLOwner)}}, ExptInfo: &entity.ExptInfo{CronActivate: true}}, Binding: bindingMySQLInput(key), Hook: entity.HookConfigUpdateInput{KeyID: "key", Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}}}
	require.NoError(t, store.Write(ctx, input), "template, encrypted config and pending binding must be created atomically")
	before, err := store.Read(ctx, key)
	require.NoError(t, err)
	require.Equal(t, "managed-original", before.Template.GetName())
	require.True(t, *before.Config.Config.Before.Enabled)
	require.False(t, before.Binding.Active())
	t.Run("late_failure_rolls_back_all", func(t *testing.T) {
		fault := errors.New("binding column failure")
		name := "template_schedule_management_fault"
		require.NoError(t, s.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
			if tx.Statement.Table == "expt_template" {
				if fields, ok := tx.Statement.Dest.(map[string]any); ok && fields["schedule_run_binding"] != nil {
					tx.AddError(fault)
				}
			}
		}))
		t.Cleanup(func() { require.NoError(t, s.Callback().Update().Remove(name)) })
		next := *before.Template
		meta := *next.Meta
		meta.Name = "must-rollback"
		next.Meta = &meta
		binding := *before.Binding
		binding.Version = 2
		binding.UserID = "new-user"
		err := store.Write(ctx, repo.ExptTemplateScheduleWrite{Key: key, ExpectedRevision: before.Revision, Template: &next, Hook: entity.HookConfigUpdateInput{ExpectedRevision: before.Config.Revision, KeyID: "key", Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}}}, Binding: &binding})
		require.Error(t, err)
		after, err := store.Read(ctx, key)
		require.NoError(t, err)
		require.Equal(t, before, after)
	})
	t.Run("concurrent_cas_and_disable_fences_receipt", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(2)
		start := make(chan struct{})
		errs := make([]error, 2)
		for i := range errs {
			go func(i int) {
				defer wg.Done()
				<-start
				next := *before.Template
				meta := *next.Meta
				meta.Name = "one-update"
				next.Meta = &meta
				binding := *before.Binding
				binding.Version = 2
				errs[i] = store.Write(ctx, repo.ExptTemplateScheduleWrite{Key: key, ExpectedRevision: before.Revision, Template: &next, Binding: &binding})
			}(i)
		}
		close(start)
		wg.Wait()
		success := 0
		for _, err := range errs {
			if err == nil {
				success++
			} else {
				require.ErrorIs(t, err, entity.ErrHookStoreConflict)
			}
		}
		require.Equal(t, 1, success)
		current, err := store.Read(ctx, key)
		require.NoError(t, err)
		require.Equal(t, int64(2), current.Binding.Version)
		require.NoError(t, store.Write(ctx, repo.ExptTemplateScheduleWrite{Key: key, ExpectedRevision: current.Revision, Fields: map[string]any{"cron_activate": false}, Disable: true}))
		_, err = bindings.ActivateCAS(ctx, key, current.Binding.BindingID, 2, bindingMySQLReceipt(key))
		require.ErrorIs(t, err, entity.ErrExptTemplateScheduleBindingConflict)
		last, err := store.Read(ctx, key)
		require.NoError(t, err)
		require.False(t, last.Binding.Enabled)
		require.Equal(t, int64(3), last.Binding.Version)
		require.False(t, last.Template.ExptInfo.CronActivate)
	})
}
