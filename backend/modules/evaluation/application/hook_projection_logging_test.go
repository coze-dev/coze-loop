// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application_test

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type projectionLoggedProvider struct {
	db.Provider
	logger logger.Interface
}

func (p projectionLoggedProvider) Transaction(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
	return p.Provider.Transaction(ctx, func(tx *gorm.DB) error { return fc(tx.Session(&gorm.Session{Logger: p.logger})) }, opts...)
}

func TestHookProjectionPersistenceDoesNotLogDisplayContent(t *testing.T) {
	f := newExecutorFixture(t, entity.HookPhaseBefore, 10)
	var logs bytes.Buffer
	f.repo = experiment.NewHookRunRepo(projectionLoggedProvider{Provider: f.p, logger: logger.New(log.New(&logs, "", 0), logger.Config{LogLevel: logger.Info})})
	_, err := projectionExecutor(t, f, 200, `{"status":"failed","error":{"code":"business-display-code-marker","message":"business-display-message-marker"}}`).Execute(context.Background(), f.input)
	require.NoError(t, err)
	require.False(t, strings.Contains(logs.String(), "business-display-code-marker"), "display code leaked into SQL logs")
	require.False(t, strings.Contains(logs.String(), "business-display-message-marker"), "display message leaked into SQL logs")
}
