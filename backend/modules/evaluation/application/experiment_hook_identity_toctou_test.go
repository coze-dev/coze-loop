// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	common "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
)

type identityFlipStore struct {
	*startStore
	flip          bool
	legacyCreates int
	legacyUser    string
}

func (s *identityFlipStore) GetConfig(ctx context.Context, owner hookcomponent.ConfigOwner) (*entity.HookConfigRecord, error) {
	record, err := s.startStore.GetConfig(ctx, owner)
	if err != nil {
		return nil, err
	}
	record.Revision = s.initial.ConfigRevision
	if s.flip && s.configReads == 1 {
		// Commit an authorized enable after the application read, before the starter's fresh read.
		s.config = enabledStartConfig()
		s.initial.HooksEnabled = true
		s.initial.ConfigRevision = "enabled-v2"
	}
	return record, nil
}
func (s *identityFlipStore) CreateRunWithoutHooks(_ context.Context, log *entity.ExptRunLog, latest int64, revision string) (bool, error) {
	if s.initial.LatestRunID != latest || s.initial.ConfigRevision != revision {
		return false, entity.ErrHookStoreConflict
	}
	s.legacyCreates++
	s.legacyUser = log.CreatedBy
	s.initial.RunLog = log
	s.initial.LatestRunID = log.ExptRunID
	s.source.LatestRunID = log.ExptRunID
	return true, nil
}

func newIdentityFlipApplication(t *testing.T, initial *entity.LifecycleHookConf, flip bool) (*experimentApplication, *identityFlipStore, *startLogs, *startQuota, *startPublisher) {
	t.Helper()
	app, base, logs, quota, publisher := newStartApplication(t, false, initial, nil)
	store := &identityFlipStore{startStore: base, flip: flip}
	identity, err := hookinfra.NewIdentityProvider(nil, 0)
	require.NoError(t, err)
	app.manager, err = service.NewExptManagerWithHooks(app.manager, service.ExptManagerHookDependencies{Initialization: store, Runs: store, Configs: store, Codec: store.codec, Identity: identity, Runtime: hookApplicationRuntime{enabled: true}, Wake: startWake{}, ExecutionScope: "test-scope", SnapshotKeyID: "operator-key"})
	require.NoError(t, err)
	app.hooks.Configs = store
	return app, store, logs, quota, publisher
}

func TestLifecycleHookIdentityConfigEnableBetweenReads(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprint(disabled), func(t *testing.T) {
			var initial *entity.LifecycleHookConf
			if disabled {
				initial = &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}}
			}
			app, store, logs, quota, publisher := newIdentityFlipApplication(t, initial, true)
			req := startRequest()
			out, err := app.RunExperiment(hookCreationContext(), req)
			require.NoError(t, err)
			require.Equal(t, int64(71), out.GetRunID())
			require.Equal(t, 2, store.configReads)
			require.Equal(t, "trusted-user", store.initial.RunLog.CreatedBy, "first disabled config read must not make body Session authoritative")
			snapshot, err := store.codec.DecodeSnapshot(context.Background(), store.run.State.Key, "test-scope", store.run.Snapshot)
			require.NoError(t, err)
			require.Equal(t, "trusted-user", snapshot.Input().Schedule.Session.UserID)
			require.Equal(t, "trusted-user", snapshot.Input().Context.Initiator.GetUserID())
			require.Equal(t, "trusted-user", publisher.sent[0].Session.UserID)
			require.Equal(t, 1, store.creates)
			require.Zero(t, store.legacyCreates)
			require.Zero(t, logs.creates)
			require.Equal(t, 1, quota.calls)
			require.Len(t, publisher.sent, 1)
			require.Equal(t, int64(999), req.Session.GetUserID())
		})
	}
}

func TestLifecycleHookIdentityMissingTrustedContextRejectsManaged(t *testing.T) {
	app, store, logs, quota, publisher := newIdentityFlipApplication(t, nil, true)
	out, err := app.RunExperiment(context.Background(), startRequest())
	require.Nil(t, out)
	require.Error(t, err)
	require.Zero(t, store.creates)
	require.Zero(t, store.legacyCreates)
	require.Zero(t, logs.creates)
	require.Zero(t, quota.calls)
	require.Empty(t, publisher.sent)
}

func TestLifecycleHookIdentityLegacySessionCompatibility(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		for _, kind := range []string{"nil", "default", "body", "missing_ctx_body"} {
			t.Run(fmt.Sprintf("disabled=%v/%s", disabled, kind), func(t *testing.T) {
				var initial *entity.LifecycleHookConf
				if disabled {
					initial = &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}}
				}
				app, store, logs, quota, publisher := newIdentityFlipApplication(t, initial, false)
				stop := errors.New("legacy Run quota storage unavailable")
				quota.err = stop
				ctx := hookCreationContext()
				req := startRequest()
				want := "trusted-user"
				switch kind {
				case "nil":
					req.Session = nil
				case "default":
					req.Session = &common.Session{}
				case "body":
					want = "999"
				case "missing_ctx_body":
					ctx = context.Background()
					want = "999"
				}
				out, err := app.RunExperiment(ctx, req)
				require.Nil(t, out)
				require.ErrorIs(t, err, stop)
				require.Equal(t, want, store.legacyUser)
				require.Equal(t, 1, store.legacyCreates)
				require.Zero(t, store.creates)
				require.Zero(t, logs.creates)
				require.Equal(t, 1, quota.calls)
				require.Empty(t, publisher.sent)
			})
		}
	}
}
