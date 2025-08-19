// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tag

import (
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/repo"
)

type TagRepoImpl struct {
	db    db.Provider
	idGen idgen.IIDGenerator
}

func NewTagRepoImpl(p db.Provider, id idgen.IIDGenerator) repo.ITagAPI {
	return &TagRepoImpl{
		db:    p,
		idGen: id,
	}
}
