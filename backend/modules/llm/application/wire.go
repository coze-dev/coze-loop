// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject
// +build wireinject

package application

import (
	"context"

	"github.com/google/wire"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth/authservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/manage"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/service/llmfactory"
	"github.com/coze-dev/coze-loop/backend/modules/llm/infra/config"
	"github.com/coze-dev/coze-loop/backend/modules/llm/infra/repo"
	"github.com/coze-dev/coze-loop/backend/modules/llm/infra/repo/dao"
	"github.com/coze-dev/coze-loop/backend/modules/llm/infra/rpc"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

var (
	llmDomainSet = wire.NewSet(
		llmfactory.NewFactory,
		config.NewManage,
		config.NewRuntime,
		service.NewRuntime,
		service.NewManage,
		repo.NewRuntimeRepo,
		dao.NewModelRequestRecordDao,
		rpc.NewAuthRPCProvider,
	)
	runtimeSet = wire.NewSet(
		NewRuntimeApplication,
		llmDomainSet,
	)
	manageSet = wire.NewSet(
		NewManageApplication,
		llmDomainSet,
	)
)

func InitRuntimeApplication(
	ctx context.Context,
	idGen idgen.IIDGenerator,
	configFactory conf.IConfigLoaderFactory,
	db db.Provider,
	redis redis.Cmdable,
	factory limiter.IRateLimiterFactory,
) (runtime.LLMRuntimeService, error) {
	wire.Build(runtimeSet)
	return nil, nil
}

func InitManageApplication(
	ctx context.Context,
	configFactory conf.IConfigLoaderFactory,
	authClient authservice.Client,
) (manage.LLMManageService, error) {
	wire.Build(manageSet)
	return nil, nil
}
