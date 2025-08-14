// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestDB(t *testing.T) {
	var (
		db      = NewTestDB(t, &somePO{})
		session = db.NewSession(context.TODO())
		id      = 1
	)

	po := &somePO{ID: id, Name: "name", Description: "desc"}
	require.NoError(t, session.Create(po).Error, "create")

	got := &somePO{}
	require.NoError(t, session.Where("id = ?", id).Find(got).Error, "select")
	assert.Equal(t, po, got)
}
