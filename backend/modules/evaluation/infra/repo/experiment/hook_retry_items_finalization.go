// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
)

func retryItemsFinalizeCursor(log model.ExptRunLog, life model.ExptLifecycleRun, intent entity.HookTerminalIntent) (string, error) {
	do, err := convert.NewExptRunLogConvertor().PO2DO(&log)
	if err != nil {
		return "", entity.ErrHookStoreCorrupt
	}
	key := entity.HookRunKey{WorkspaceID: life.SpaceID, ExperimentID: life.ExptID, RunID: life.ExptRunID}
	raw, hash := gptr.Indirect(life.PlanCursor), gptr.Indirect(life.PlanHash)
	if intent.Status == entity.ExptStatus_Terminated || intent.Status == entity.ExptStatus_SystemTerminated {
		return entity.CloseHookRetryItemsCursor(raw, key, do.ItemIds, life.PlanCount, hash, intent.Reason)
	}
	if !life.ExecutionInitialized || life.PlanState != 1 {
		return "", entity.ErrHookFinalizationUnsettled
	}
	c, err := entity.DecodeHookRetryItemsCursor(raw, key, do.ItemIds, life.PlanCount, hash)
	if err != nil {
		return "", err
	}
	if c.Phase != "tail" || c.Batch != len(do.ItemIds) || c.Offset != 0 {
		return "", entity.ErrHookFinalizationUnsettled
	}
	return raw, nil
}

func checkRetryItemsFinalizationCoverage(tx *gorm.DB, key entity.HookRunKey, life model.ExptLifecycleRun) error {
	var log model.ExptRunLog
	if err := hookRunScope(tx.Unscoped(), key).First(&log).Error; err != nil {
		return err
	}
	if !hookCancellation(&life) {
		_, err := retryItemsFinalizeCursor(log, life, entity.HookTerminalIntent{Status: entity.ExptStatus_Success})
		return err
	}
	do, err := convert.NewExptRunLogConvertor().PO2DO(&log)
	if err != nil {
		return entity.ErrHookStoreCorrupt
	}
	c, err := entity.DecodeHookRetryItemsCursor(gptr.Indirect(life.PlanCursor), key, do.ItemIds, life.PlanCount, gptr.Indirect(life.PlanHash))
	if err != nil {
		return err
	}
	if c.Terminal == nil || c.Terminal.Reason != gptr.Indirect(life.TerminalReason) {
		return entity.ErrHookFinalizationUnsettled
	}
	return nil
}
