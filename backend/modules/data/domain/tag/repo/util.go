// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

func TagKeyOrderBy(orderBy string) pagination.PaginatorOption {
	switch strings.ToLower(orderBy) {
	case "", pagination.ColumnUpdatedAt:
		return pagination.WithOrderBy(pagination.ColumnUpdatedAt, pagination.ColumnID)
	case pagination.ColumnCreatedAt:
		return pagination.WithOrderBy(pagination.ColumnCreatedAt, pagination.ColumnID)
	default:
		return pagination.WithError(errors.Errorf("invalid order_by '%s', supported=%s and %s", orderBy, pagination.ColumnUpdatedAt, pagination.ColumnCreatedAt))
	}
}
