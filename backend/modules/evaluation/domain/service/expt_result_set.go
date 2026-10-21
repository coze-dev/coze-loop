// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func (e *ExptMangerImpl) checkResultSetAdmission(ctx context.Context, spaceID int64, conf *entity.EvaluationConfiguration) error {
	if !conf.IsResultSet() {
		return nil
	}
	gate, ok := e.configer.(component.ResultSetAdmission)
	if !ok {
		return fmt.Errorf("result_set is disabled in this deployment")
	}
	versions, err := conf.ResultSetBinaryVersions()
	if err != nil {
		return err
	}
	for _, version := range versions {
		if err := gate.CheckResultSetRuntime(ctx, spaceID, version); err != nil {
			return err
		}
	}
	return nil
}
