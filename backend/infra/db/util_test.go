// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWhereBuilder(t *testing.T) {
	b := NewWhereBuilder()
	b.WithIndex()

	MaybeAddEqToWhere(b, 1, "col_1")
	MaybeAddEqToWhere(b, 0, "col_2") // won't be added

	MaybeAddInToWhere(b, []int{1, 2, 3}, "col_3")
	MaybeAddInToWhere(b, []int{}, "col_4") // won't be added

	MaybeAddGtToWhere(b, 10, "col_5")
	MaybeAddGtToWhere(b, 0, "col_6") // won't be added

	MaybeAddLteToWhere(b, 12, "col_7")
	MaybeAddLteToWhere(b, 0, "col_8") // won't be added

	MaybeAddLikeToWhere(b, "x", "col_9")
	MaybeAddLikeToWhere(b, "", "col_10") // won't be added

	where, err := b.Build()
	assert.NoError(t, err)

	p := NewTestDB(t, &somePO{})
	session := p.NewSession(context.TODO())
	session.DryRun = true
	res := make(map[string]any)
	got := session.Select("*").Table(`x`).Where(where).Find(&res).Statement.SQL.String()
	want := "SELECT * FROM `x` WHERE `col_1` = ? AND `col_3` IN (?,?,?) AND `col_5` > ? AND `col_7` <= ? AND col_9 LIKE ? ESCAPE '\\\\'"
	assert.Equal(t, want, got)
}
