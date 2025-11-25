// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"github.com/google/wire"
	"github.com/sirupsen/logrus"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

var RuntimeSet = wire.NewSet(
	NewSandboxConfig,
	NewLogger,
	NewRuntimeFactory,
	NewRuntimeManagerFromFactory,
)

func NewSandboxConfig() *entity.SandboxConfig {
	return entity.DefaultSandboxConfig()
}

func NewLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	return logger
}

func NewRuntimeManagerFromFactory(factory component.IRuntimeFactory, logger *logrus.Logger) component.IRuntimeManager {
	return NewRuntimeManager(factory, logger)
}
