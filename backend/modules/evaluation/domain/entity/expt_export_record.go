// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strconv"
	"time"
)

type ExptResultExportRecord struct {
	ID              int64
	SpaceID         int64
	ExptID          int64
	CsvExportStatus CSVExportStatus
	FilePath        string
	CreatedBy       string
	URL             *string
	Expired         bool
	ErrMsg          string

	StartAt *time.Time
	EndAt   *time.Time
}

type CSVExportStatus int32

const (
	CSVExportStatus_Unknown CSVExportStatus = 0

	CSVExportStatus_Running CSVExportStatus = 1
	CSVExportStatus_Success CSVExportStatus = 2
	CSVExportStatus_Failed  CSVExportStatus = 3
)

func DefaultExptExportWhiteList() *ExptExportWhiteList {
	return &ExptExportWhiteList{}
}

type ExptExportWhiteList struct {
	UserIDs []int64 `json:"user_ids" mapstructure:"user_ids"`
}

func (e *ExptExportWhiteList) IsUserIDInWhiteList(userID string) bool {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return false
	}
	for _, id := range e.UserIDs {
		if id == uid {
			return true
		}
	}
	return false
}
