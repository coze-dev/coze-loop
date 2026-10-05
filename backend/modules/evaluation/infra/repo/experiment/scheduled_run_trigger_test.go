// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var scheduledTriggerSequence = func() *atomic.Int64 { n := new(atomic.Int64); n.Store(time.Now().UnixNano()); return n }()

type scheduledTriggerFixture struct {
	p   db.Provider
	sql *gorm.DB
	r   repo.IScheduledRunTriggerRepo
	b   entity.ExptTemplateScheduleBinding
	ids []entity.ScheduledRunTriggerIDs
}

func newScheduledTriggerFixture(t *testing.T) *scheduledTriggerFixture {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires explicitly configured HOOK_MYSQL_TX_DSN; not MySQL evidence")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	require.True(t, cfg.ParseTime)
	require.Equal(t, "UTC", cfg.Loc.String())
	require.Equal(t, "'+00:00'", cfg.Params["time_zone"])
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	s := p.NewSession(context.Background(), db.WithMaster())
	conn, err := s.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(12)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	var actual struct {
		DatabaseName, Socket, Version string
		SkipNetworking                int
	}
	require.NoError(t, s.Raw("SELECT DATABASE() AS database_name, @@socket AS socket, VERSION() AS version, @@skip_networking AS skip_networking").Scan(&actual).Error)
	require.Equal(t, cfg.DBName, actual.DatabaseName)
	require.Equal(t, cfg.Addr, actual.Socket)
	require.Equal(t, 1, actual.SkipNetworking)
	t.Logf("MYSQL_ENABLED database=%s version=%s socket=%s", actual.DatabaseName, actual.Version, actual.Socket)
	for _, table := range []string{"expt_template", "expt_template_trigger", "experiment", "expt_run_log"} {
		var engine string
		require.NoError(t, s.Raw("SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?", table).Scan(&engine).Error)
		require.Equal(t, "InnoDB", engine, table)
	}
	f := &scheduledTriggerFixture{p: p, sql: s, r: NewScheduledRunTriggerRepo(p)}
	f.b = entity.ExptTemplateScheduleBinding{SchemaVersion: 1, BindingID: fmt.Sprintf("scheduled-trigger-%d", scheduledTriggerSequence.Add(1)), Version: 1,
		UserID: "scheduled-trigger-bound-user", IdentityType: "fornax_user", SpaceID: scheduledTriggerSequence.Add(1), TemplateID: scheduledTriggerSequence.Add(1),
		ExecutionScope: "local", Namespace: "ns", Group: "group", BizKey: "key", JobID: "job", Enabled: true, BoundAt: time.Unix(1700000000, 0).UTC(),
		Callback: entity.ExptTemplateScheduleCallback{PSM: "test.evaluation", Method: "SubmitScheduledExptFromTemplate", Cluster: "default"}}
	raw, err := json.Marshal(f.b)
	require.NoError(t, err)
	require.NoError(t, s.Create(&model.ExptTemplate{ID: f.b.TemplateID, SpaceID: f.b.SpaceID, Name: f.b.BindingID, CreatedBy: "scheduled-trigger-fixture", CronActivate: true, ScheduleRunBinding: &raw}).Error)
	t.Cleanup(func() {
		q := p.NewSession(context.Background(), db.WithMaster())
		for _, ids := range f.ids {
			require.NoError(t, q.Exec("DELETE FROM expt_run_log WHERE id=? AND expt_run_id=? AND expt_id=? AND space_id=?", ids.RunID, ids.RunID, ids.ExperimentID, f.b.SpaceID).Error)
			require.NoError(t, q.Exec("DELETE FROM experiment WHERE id=? AND space_id=? AND expt_template_id=?", ids.ExperimentID, f.b.SpaceID, f.b.TemplateID).Error)
		}
		require.NoError(t, q.Exec("DELETE FROM expt_template_trigger WHERE binding_id=? AND space_id=? AND template_id=?", f.b.BindingID, f.b.SpaceID, f.b.TemplateID).Error)
		require.NoError(t, q.Exec("DELETE FROM expt_template WHERE id=? AND space_id=? AND created_by=?", f.b.TemplateID, f.b.SpaceID, "scheduled-trigger-fixture").Error)
		var left int64
		require.NoError(t, q.Table("expt_template_trigger").Where("binding_id=? AND space_id=? AND template_id=?", f.b.BindingID, f.b.SpaceID, f.b.TemplateID).Count(&left).Error)
		require.Zero(t, left)
		t.Logf("FIXTURE_CLEANED binding=%s space=%d template=%d remaining_triggers=%d", f.b.BindingID, f.b.SpaceID, f.b.TemplateID, left)
	})
	return f
}

func (f *scheduledTriggerFixture) allocate() entity.ScheduledRunTriggerIDs {
	ids := entity.ScheduledRunTriggerIDs{TriggerID: scheduledTriggerSequence.Add(1), ExperimentID: scheduledTriggerSequence.Add(1), RunID: scheduledTriggerSequence.Add(1)}
	f.ids = append(f.ids, ids)
	return ids
}

func scheduledTriggerWrite(ctx context.Context, trigger entity.ScheduledRunTrigger, user string, p db.Provider) error {
	return p.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(&model.Experiment{ID: trigger.ExperimentID, SpaceID: trigger.SpaceID, Name: fmt.Sprint(trigger.ExperimentID), CreatedBy: user, ExptTemplateID: trigger.TemplateID, LatestRunID: trigger.RunID}).Error; err != nil {
			return err
		}
		return tx.Create(&model.ExptRunLog{ID: trigger.RunID, SpaceID: trigger.SpaceID, ExptID: trigger.ExperimentID, ExptRunID: trigger.RunID, CreatedBy: user}).Error
	})
}

func (f *scheduledTriggerFixture) counts(t *testing.T, status string, expts, runs int64) {
	t.Helper()
	var row model.ExptTemplateTrigger
	require.NoError(t, f.sql.Where("binding_id=? AND space_id=? AND template_id=?", f.b.BindingID, f.b.SpaceID, f.b.TemplateID).Take(&row).Error)
	require.Equal(t, status, row.Status)
	var count int64
	require.NoError(t, f.sql.Table("experiment").Where("space_id=? AND expt_template_id=?", f.b.SpaceID, f.b.TemplateID).Count(&count).Error)
	require.Equal(t, expts, count)
	require.NoError(t, f.sql.Table("expt_run_log").Where("space_id=?", f.b.SpaceID).Count(&count).Error)
	require.Equal(t, runs, count)
}

func TestScheduledTriggerMySQLReserveReplayAndConcurrent(t *testing.T) {
	f := newScheduledTriggerFixture(t)
	ctx := context.Background()
	ids := f.allocate()
	first, err := f.r.Reserve(ctx, &f.b, "instance-1", ids)
	require.NoError(t, err)
	require.Equal(t, ids.TriggerID, first.ID)
	require.Equal(t, ids.ExperimentID, first.ExperimentID)
	require.Equal(t, ids.RunID, first.RunID)
	require.Equal(t, entity.ScheduledRunTriggerPending, first.Status)
	again, err := f.r.Reserve(ctx, &f.b, "instance-1", f.allocate())
	require.NoError(t, err)
	require.Equal(t, first, again)
	candidates := make([]entity.ScheduledRunTriggerIDs, 8)
	for i := range candidates {
		candidates[i] = f.allocate()
	}
	type result struct {
		value *entity.ScheduledRunTrigger
		err   error
	}
	done := make(chan result, len(candidates))
	start := make(chan struct{})
	for _, candidate := range candidates {
		go func(ids entity.ScheduledRunTriggerIDs) {
			<-start
			v, e := f.r.Reserve(ctx, &f.b, "instance-race", ids)
			done <- result{v, e}
		}(candidate)
	}
	close(start)
	var winner *entity.ScheduledRunTrigger
	for range candidates {
		r := <-done
		require.NoError(t, r.err)
		if winner == nil {
			winner = r.value
		}
		require.Equal(t, winner, r.value)
	}
	var n int64
	require.NoError(t, f.sql.Table("expt_template_trigger").Where("binding_id=?", f.b.BindingID).Count(&n).Error)
	require.EqualValues(t, 2, n)
}

func TestScheduledTriggerMySQLRejectsUntrustedOrChangedBinding(t *testing.T) {
	for _, field := range []string{"user", "version", "scope", "job", "namespace", "group", "biz_key", "callback", "bound_at", "disabled", "cron_off", "missing_binding", "corrupt_binding"} {
		t.Run(field, func(t *testing.T) {
			f := newScheduledTriggerFixture(t)
			expected := f.b
			switch field {
			case "user":
				expected.UserID = "body-user"
			case "version":
				expected.Version++
			case "scope":
				expected.ExecutionScope = "other"
			case "job":
				expected.JobID = "other"
			case "namespace":
				expected.Namespace = "other"
			case "group":
				expected.Group = "other"
			case "biz_key":
				expected.BizKey = "other"
			case "callback":
				expected.Callback.Method = "Other"
			case "bound_at":
				expected.BoundAt = expected.BoundAt.Add(time.Second)
			case "disabled":
				_, err := NewExptTemplateScheduleBindingRepo(f.p).DisableCAS(context.Background(), entity.ExptTemplateScheduleBindingKey{SpaceID: f.b.SpaceID, TemplateID: f.b.TemplateID, ExecutionScope: f.b.ExecutionScope}, f.b.BindingID, f.b.Version)
				require.NoError(t, err)
			case "cron_off":
				require.NoError(t, f.sql.Exec("UPDATE expt_template SET cron_activate=0 WHERE id=? AND space_id=?", f.b.TemplateID, f.b.SpaceID).Error)
			case "missing_binding":
				require.NoError(t, f.sql.Exec("UPDATE expt_template SET schedule_run_binding=NULL WHERE id=? AND space_id=?", f.b.TemplateID, f.b.SpaceID).Error)
			case "corrupt_binding":
				require.NoError(t, f.sql.Exec("UPDATE expt_template SET schedule_run_binding=? WHERE id=? AND space_id=?", []byte(`{"bad":true}`), f.b.TemplateID, f.b.SpaceID).Error)
			}
			got, err := f.r.Reserve(context.Background(), &expected, "instance", f.allocate())
			require.Error(t, err)
			require.Nil(t, got)
			var n int64
			require.NoError(t, f.sql.Table("expt_template_trigger").Where("space_id=?", f.b.SpaceID).Count(&n).Error)
			require.Zero(t, n)
		})
	}
}

func TestScheduledTriggerMySQLCommitRollbackAndReplay(t *testing.T) {
	f := newScheduledTriggerFixture(t)
	ctx := context.Background()
	first, err := f.r.Reserve(ctx, &f.b, "instance", f.allocate())
	require.NoError(t, err)
	failure := errors.New("prepared SQL failed")
	got, err := f.r.Commit(ctx, &f.b, "instance", func(ctx context.Context, tr entity.ScheduledRunTrigger, user string, p db.Provider) error {
		require.Equal(t, f.b.UserID, user)
		if err := scheduledTriggerWrite(ctx, tr, user, p); err != nil {
			return err
		}
		return failure
	})
	require.ErrorIs(t, err, failure)
	require.Nil(t, got)
	f.counts(t, "pending", 0, 0)
	committed, err := f.r.Commit(ctx, &f.b, "instance", scheduledTriggerWrite)
	require.NoError(t, err)
	require.Equal(t, first.ID, committed.ID)
	require.Equal(t, first.ExperimentID, committed.ExperimentID)
	require.Equal(t, first.RunID, committed.RunID)
	require.Equal(t, entity.ScheduledRunTriggerSubmitted, committed.Status)
	f.counts(t, "submitted", 1, 1)
	replay, err := f.r.Commit(ctx, &f.b, "instance", func(context.Context, entity.ScheduledRunTrigger, string, db.Provider) error {
		t.Fatal("submitted intent executed twice")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, committed, replay)
	replay, err = f.r.Reserve(ctx, &f.b, "instance", f.allocate())
	require.NoError(t, err)
	require.Equal(t, committed, replay)
}

func TestScheduledTriggerMySQLCommitRequiresActualBoundRun(t *testing.T) {
	for _, mode := range []string{"no_op", "wrong_user", "user_case", "binding_change", "trigger_ids", "status_write_failure"} {
		t.Run(mode, func(t *testing.T) {
			f := newScheduledTriggerFixture(t)
			ctx := context.Background()
			_, err := f.r.Reserve(ctx, &f.b, "instance", f.allocate())
			require.NoError(t, err)
			if mode == "status_write_failure" {
				name := "scheduled-trigger-fail-status"
				require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
					if tx.Statement.Table == "expt_template_trigger" {
						tx.AddError(errors.New("status write failed"))
					}
				}))
				t.Cleanup(func() { require.NoError(t, f.sql.Callback().Update().Remove(name)) })
			}
			got, err := f.r.Commit(ctx, &f.b, "instance", func(ctx context.Context, tr entity.ScheduledRunTrigger, user string, p db.Provider) error {
				if mode == "no_op" {
					return nil
				}
				if mode == "wrong_user" {
					user = "body-user"
				}
				if mode == "user_case" {
					user = strings.ToUpper(user)
				}
				if err := scheduledTriggerWrite(ctx, tr, user, p); err != nil {
					return err
				}
				if mode == "binding_change" {
					return p.NewSession(ctx).Exec("UPDATE expt_template SET cron_activate=0 WHERE id=? AND space_id=?", f.b.TemplateID, f.b.SpaceID).Error
				}
				if mode == "trigger_ids" {
					return p.NewSession(ctx).Exec("UPDATE expt_template_trigger SET expt_run_id=expt_run_id+1 WHERE id=? AND space_id=?", tr.ID, tr.SpaceID).Error
				}
				return nil
			})
			require.Error(t, err)
			require.Nil(t, got)
			f.counts(t, "pending", 0, 0)
		})
	}
}

func TestScheduledTriggerMySQLConcurrentCommit(t *testing.T) {
	f := newScheduledTriggerFixture(t)
	ctx := context.Background()
	_, err := f.r.Reserve(ctx, &f.b, "instance", f.allocate())
	require.NoError(t, err)
	var calls atomic.Int32
	done := make(chan error, 8)
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			<-start
			_, err := f.r.Commit(ctx, &f.b, "instance", func(ctx context.Context, tr entity.ScheduledRunTrigger, user string, p db.Provider) error {
				calls.Add(1)
				return scheduledTriggerWrite(ctx, tr, user, p)
			})
			done <- err
		}()
	}
	close(start)
	for i := 0; i < 8; i++ {
		require.NoError(t, <-done)
	}
	require.EqualValues(t, 1, calls.Load())
	f.counts(t, "submitted", 1, 1)
}

func TestScheduledTriggerMySQLBindingRace(t *testing.T) {
	for _, first := range []string{"disable", "version", "commit"} {
		t.Run(first, func(t *testing.T) {
			f := newScheduledTriggerFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := f.r.Reserve(ctx, &f.b, "instance", f.allocate())
			require.NoError(t, err)
			key := entity.ExptTemplateScheduleBindingKey{SpaceID: f.b.SpaceID, TemplateID: f.b.TemplateID, ExecutionScope: f.b.ExecutionScope}
			locked, release := make(chan struct{}), make(chan struct{})
			disableDone, commitDone := make(chan error, 1), make(chan error, 1)
			if first != "commit" {
				go func() {
					disableDone <- f.p.Transaction(ctx, func(tx *gorm.DB) error {
						bindings := NewExptTemplateScheduleBindingRepo(f.p)
						var e error
						if first == "disable" {
							_, e = bindings.DisableCAS(ctx, key, f.b.BindingID, f.b.Version, db.WithTransaction(tx))
						} else {
							next := f.b
							next.Version++
							next.JobID = ""
							_, e = bindings.SaveCAS(ctx, key, f.b.Version, &next, db.WithTransaction(tx))
							if e == nil {
								_, e = bindings.ActivateCAS(ctx, key, next.BindingID, next.Version, entity.ExptTemplateScheduleReceipt{JobID: f.b.JobID, Namespace: f.b.Namespace, Group: f.b.Group, BizKey: f.b.BizKey, Callback: f.b.Callback}, db.WithTransaction(tx))
							}
						}
						close(locked)
						<-release
						return e
					})
				}()
				<-locked
				go func() { _, e := f.r.Commit(ctx, &f.b, "instance", scheduledTriggerWrite); commitDone <- e }()
				select {
				case e := <-commitDone:
					t.Fatalf("commit escaped template lock: %v", e)
				case <-time.After(80 * time.Millisecond):
				}
				close(release)
				require.NoError(t, <-disableDone)
				require.Error(t, <-commitDone)
				f.counts(t, "pending", 0, 0)
			} else {
				go func() {
					_, e := f.r.Commit(ctx, &f.b, "instance", func(ctx context.Context, tr entity.ScheduledRunTrigger, user string, p db.Provider) error {
						close(locked)
						<-release
						return scheduledTriggerWrite(ctx, tr, user, p)
					})
					commitDone <- e
				}()
				<-locked
				go func() {
					_, e := NewExptTemplateScheduleBindingRepo(f.p).DisableCAS(ctx, key, f.b.BindingID, f.b.Version)
					disableDone <- e
				}()
				select {
				case e := <-disableDone:
					t.Fatalf("disable escaped commit lock: %v", e)
				case <-time.After(80 * time.Millisecond):
				}
				close(release)
				require.NoError(t, <-commitDone)
				require.NoError(t, <-disableDone)
				f.counts(t, "submitted", 1, 1)
				_, err = f.r.Reserve(ctx, &f.b, "instance", f.allocate())
				require.Error(t, err)
			}
		})
	}
}

func TestScheduledTriggerMySQLInvalidInputAndCorruptIntent(t *testing.T) {
	f := newScheduledTriggerFixture(t)
	ctx := context.Background()
	for _, instance := range []string{"", strings.Repeat("x", 129), "a\x00b", " a"} {
		_, err := f.r.Reserve(ctx, &f.b, instance, f.allocate())
		require.Error(t, err)
	}
	_, err := f.r.Reserve(ctx, &f.b, "instance", entity.ScheduledRunTriggerIDs{})
	require.Error(t, err)
	_, err = f.r.Reserve(ctx, nil, "instance", f.allocate())
	require.Error(t, err)
	_, err = f.r.Commit(ctx, &f.b, "missing", scheduledTriggerWrite)
	require.Error(t, err)
	_, err = f.r.Commit(ctx, &f.b, "missing", nil)
	require.Error(t, err)
	_, err = f.r.Reserve(ctx, &f.b, "instance", f.allocate())
	require.NoError(t, err)
	require.NoError(t, f.sql.Exec("UPDATE expt_template_trigger SET status='unknown' WHERE binding_id=? AND space_id=?", f.b.BindingID, f.b.SpaceID).Error)
	_, err = f.r.Reserve(ctx, &f.b, "instance", f.allocate())
	require.Error(t, err)
	_, err = f.r.Commit(ctx, &f.b, "instance", scheduledTriggerWrite)
	require.Error(t, err)
}

type scheduledTriggerLostAck struct {
	db.Provider
	err error
}

func (p scheduledTriggerLostAck) Transaction(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
	if err := p.Provider.Transaction(ctx, fn, opts...); err != nil {
		return err
	}
	return p.err
}

func TestScheduledTriggerMySQLLostCommitReceipts(t *testing.T) {
	f := newScheduledTriggerFixture(t)
	ctx := context.Background()
	ids := f.allocate()
	lost := errors.New("commit succeeded but acknowledgment lost")
	r := NewScheduledRunTriggerRepo(scheduledTriggerLostAck{Provider: f.p, err: lost})
	got, err := r.Reserve(ctx, &f.b, "instance", ids)
	require.ErrorIs(t, err, lost)
	require.Nil(t, got)
	read, err := f.r.Reserve(ctx, &f.b, "instance", f.allocate())
	require.NoError(t, err)
	require.Equal(t, ids.TriggerID, read.ID)
	require.Equal(t, ids.ExperimentID, read.ExperimentID)
	require.Equal(t, ids.RunID, read.RunID)
	got, err = r.Commit(ctx, &f.b, "instance", scheduledTriggerWrite)
	require.ErrorIs(t, err, lost)
	require.Nil(t, got)
	f.counts(t, "submitted", 1, 1)
	replayed, err := f.r.Commit(ctx, &f.b, "instance", func(context.Context, entity.ScheduledRunTrigger, string, db.Provider) error {
		t.Fatal("lost acknowledgment caused a second submission")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, ids.RunID, replayed.RunID)
	require.Equal(t, entity.ScheduledRunTriggerSubmitted, replayed.Status)
}
