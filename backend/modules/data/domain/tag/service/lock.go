// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"
)

func FormatCreateTagKey(spaceID int64, tagName string) string {
	return fmt.Sprintf("create_tag_key_%d_%s", spaceID, tagName)
}

func FormatUpdateTagKey(spaceID, tagKeyID int64) string {
	return fmt.Sprintf("update_tag_key_%d_%d", spaceID, tagKeyID)
}
