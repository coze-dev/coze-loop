// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"github.com/google/wire"
)

var ExperimentCKDAOSet = wire.NewSet(
	NewExptTurnResultFilterDAO,
)
