// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"fmt"
	"strings"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/server"
	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/mysql_db"
	vmysql "github.com/dolthub/vitess/go/mysql"
	"github.com/pkg/errors"
)

// NewTestDB 创建一个测试用的 db 实例, 创建失败会导致测试失败
// 提供的 pos 将会自动执行 migration.
func NewTestDB(t tester, pos ...any) Provider {
	d, err := newTestInMemDB(t, pos...)
	if err != nil {
		t.Errorf("new test db failed, err=%v", err)
		return nil
	}
	return d
}

type tester interface {
	Errorf(format string, args ...interface{})
	Cleanup(func())
}

func newTestInMemDB(t tester, pos ...any) (Provider, error) {
	// Start in-memory server.
	d, err := newInMemDB()
	if err != nil {
		return nil, errors.WithMessage(err, "new in-memory mysql")
	}
	d.start()

	p, err := NewDBFromConfig(d.cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "open in-memory mysql")
	}

	db := p.NewSession(context.TODO())

	// Auto migrate.
	if err := db.AutoMigrate(pos...); err != nil {
		return nil, errors.WithMessage(err, "auto migrate")
	}

	t.Cleanup(func() {
		if err := d.close(); err != nil {
			t.Errorf("close in-memory mysql failed, err=%v", err)
		}
	})

	return p, nil
}

type inMemDB struct {
	s       *server.Server
	cfg     *Config
	done    chan struct{}
	errChan chan error
}

func newInMemDB() (*inMemDB, error) {
	cfg := &Config{
		DBName:     "test",
		DBHostname: "127.0.0.1",
		DBPort:     "0", // random port
	}
	s, err := newInMemServer(cfg)
	if err != nil {
		return nil, err
	}

	d := &inMemDB{
		s:       s,
		done:    make(chan struct{}),
		errChan: make(chan error, 1),
		cfg:     cfg,
	}

	addr := d.s.Listener.Addr().String()
	if ws := strings.Split(addr, ":"); len(ws) > 1 {
		// Write actual DBPort back.
		cfg.DBPort = ws[len(ws)-1]
	}
	return d, nil
}

func newInMemServer(cfg *Config) (*server.Server, error) {
	// BuildItemResults config.
	config := server.Config{
		Protocol: "tcp",
		Address:  fmt.Sprintf("%s:%s", cfg.DBHostname, cfg.DBPort),
	}

	// BuildItemResults engine.
	db := memory.NewDatabase(cfg.DBName)
	db.EnablePrimaryKeyIndexes()
	provider := memory.NewDBProvider(db)
	engine := sqle.NewDefault(provider)

	// BuildItemResults session.
	builder := func(ctx context.Context, conn *vmysql.Conn, addr string) (sql.Session, error) {
		host := config.Address
		user := cfg.User
		mysqlConnectionUser, ok := conn.UserData.(mysql_db.MysqlConnectionUser)
		if ok {
			host = mysqlConnectionUser.Host
			user = mysqlConnectionUser.User
		}

		client := sql.Client{Address: host, User: user, Capabilities: conn.Capabilities}
		baseSession := sql.NewBaseSessionWithClientServer(addr, client, conn.ConnectionID)
		return memory.NewSession(baseSession, provider), nil
	}

	// New MySQL server.
	s, err := server.NewServer(config, engine, builder, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "new in-memory mysql server")
	}

	return s, nil
}

func (d *inMemDB) start() {
	go func() {
		defer func() { close(d.done) }()

		if err := d.s.Start(); err != nil {
			d.errChan <- errors.WithMessage(err, "start in-memory mysql server")
		}
	}()
}

func (d *inMemDB) close() error {
	select {
	case err := <-d.errChan:
		if err != nil {
			return err
		}

	default:
		if err := d.s.Close(); err != nil {
			return errors.WithMessage(err, "close server")
		}
	}

	<-d.done
	return nil
}
