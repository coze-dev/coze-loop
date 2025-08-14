// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBWithMaster(t *testing.T) {
	db := NewTestDB(t, &somePO{})
	replicaDB := NewTestDB(t, &somePO{})

	// configure dbresolver
	main := db.(*provider).db
	replica := replicaDB.(*provider).db
	_ = main.Use(dbresolver.Register(dbresolver.Config{
		Sources:  []gorm.Dialector{main.Dialector},
		Replicas: []gorm.Dialector{replica.Dialector},
	}))

	// write to main
	ctx := context.TODO()
	s1 := db.NewSession(ctx)
	require.NoError(t, s1.Create(&somePO{ID: 101, Name: "test", Description: "test"}).Error)

	// read from replica, got nothing
	s2 := db.NewSession(ctx)
	po := &somePO{}
	assert.ErrorIs(t, s2.First(po).Error, gorm.ErrRecordNotFound)

	// read with master
	s3 := db.NewSession(ctx, WithMaster())
	assert.NoError(t, s3.First(po).Error)
	assert.Equal(t, "test", po.Name)
}

func TestDBTransaction(t *testing.T) {
	db := NewTestDB(t, &somePO{})
	ctx := context.TODO()

	id := 101
	createPO := func(opts ...Option) error {
		in := &somePO{
			ID:          id,
			Name:        "test",
			Description: "test",
		}
		return db.NewSession(ctx, opts...).Create(in).Error
	}
	getPO := func(opts ...Option) error {
		got := &somePO{}
		return db.NewSession(ctx, opts...).Where("id = ?", id).First(got).Error
	}

	err := db.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []Option{WithTransaction(tx)}
		if err := createPO(opts...); err != nil {
			return err
		}
		if err := getPO(opts...); err != nil {
			return err
		}
		return errors.New("some error in transaction")
	})

	assert.Error(t, err)
	assert.ErrorIs(t, getPO(), gorm.ErrRecordNotFound)
}

func TestDBWithSelectForUpdate(t *testing.T) {
	db := NewTestDB(t, &somePO{})
	ctx := context.TODO()
	po := &somePO{ID: 101, Name: "test", Description: "test"}
	session := db.NewSession(ctx, WithSelectForUpdate())
	session.DryRun = true
	// Notice: the test db driver does not support row level lock, so we check the sql string only.
	got := session.Where("id = ?", po.ID).First(po).Statement.SQL.String()
	assert.Contains(t, got, "FOR UPDATE")
}

func TestDBWithDeleted(t *testing.T) {
	db := NewTestDB(t, &somePO{})
	ctx := context.TODO()
	po := &somePO{ID: 101, Name: "test", Description: "test"}
	session := db.NewSession(ctx)
	assert.NoError(t, session.Create(po).Error)
	assert.NoError(t, session.Delete(po).Error)
	assert.ErrorIs(t, session.First(po).Error, gorm.ErrRecordNotFound)

	session = db.NewSession(ctx, WithDeleted())
	assert.NoError(t, session.First(po).Error)
}

type mockLogWriter struct {
	called int
}

func (m *mockLogWriter) Printf(string, ...interface{}) {
	m.called++
}

func TestDBDebug(t *testing.T) {
	db, err := newInMemDB()
	require.NoError(t, err)
	db.start()
	t.Cleanup(func() { _ = db.close() })

	w := &mockLogWriter{}
	l := logger.New(w, logger.Config{LogLevel: logger.Silent})
	p, err := NewDBFromConfig(db.cfg, &gorm.Config{Logger: l}) // this option does not work
	require.NoError(t, err)
	// hack logger
	p.(*provider).db.Logger = l

	ctx := context.TODO()
	s1 := p.NewSession(ctx)
	s1.First(&somePO{})
	assert.Equal(t, 0, w.called)

	s2 := p.NewSession(ctx, Debug())
	s2.First(&somePO{})
	assert.Equal(t, 1, w.called)
}
