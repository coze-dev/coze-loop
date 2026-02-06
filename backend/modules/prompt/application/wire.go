// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

//go:build wireinject
// +build wireinject

package application

import (
	"github.com/google/wire"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/auth/authservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file/fileservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/user/userservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime/llmruntimeservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/debug"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/execute"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/manage"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/collector"
	promptconf "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/conf"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/hooks"
	rediscache "github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/redis"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/rpc"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

var (
	promptDomainSet = wire.NewSet(
		service.NewPromptFormatter,
		service.NewToolConfigProvider,
		service.NewToolResultsCollector,
		service.NewPromptService,
		repo.NewManageRepo,
		repo.NewLabelRepo,
		repo.NewDebugLogRepo,
		repo.NewDebugContextRepo,
		hooks.NewEmptyPromptCommitHook,
		hooks.NewEmptyPromptUserDraftHook,
		mysql.NewPromptBasicDAO,
		mysql.NewPromptCommitDAO,
		mysql.NewPromptUserDraftDAO,
		mysql.NewPromptRelationDAO,
		mysql.NewLabelDAO,
		mysql.NewCommitLabelMappingDAO,
		mysql.NewDebugLogDAO,
		mysql.NewDebugContextDAO,
		rediscache.NewPromptBasicDAO,
		rediscache.NewPromptDAO,
		rediscache.NewPromptLabelVersionDAO,
		promptconf.NewPromptConfigProvider,
		rpc.NewLLMRPCProvider,
		rpc.NewAuthRPCProvider,
		rpc.NewFileRPCProvider,
		rpc.NewUserRPCProvider,
		rpc.NewAuditRPCProvider,
		collector.NewEventCollectorProvider,
		service.NewCozeLoopSnippetParser,
	)
	manageSet = wire.NewSet(
		NewPromptManageApplication,
		promptDomainSet,
	)
	debugSet = wire.NewSet(
		NewPromptDebugApplication,
		promptDomainSet,
	)
	executeSet = wire.NewSet(
		NewPromptExecuteApplication,
		promptDomainSet,
	)
	openAPISet = wire.NewSet(
		NewPromptOpenAPIApplication,
		promptDomainSet,
	)
)

func InitPromptManageApplication(
	idgen idgen.IIDGenerator,
	db db.Provider,
	redisCli redis.Cmdable,
	meter metrics.Meter,
	configFactory conf.IConfigLoaderFactory,
	llmClient llmruntimeservice.Client,
	authClient authservice.Client,
	fileClient fileservice.Client,
	userClient userservice.Client,
	auditClient audit.IAuditService,
) (manage.PromptManageService, error) {
	wire.Build(manageSet)
	return nil, nil
}

func InitPromptDebugApplication(
	idgen idgen.IIDGenerator,
	db db.Provider,
	redisCli redis.Cmdable,
	meter metrics.Meter,
	configFactory conf.IConfigLoaderFactory,
	llmClient llmruntimeservice.Client,
	authClient authservice.Client,
	fileClient fileservice.Client,
	benefitSvc benefit.IBenefitService,
) (debug.PromptDebugService, error) {
	wire.Build(debugSet)
	return nil, nil
}

func InitPromptExecuteApplication(
	idgen idgen.IIDGenerator,
	db db.Provider,
	redisCli redis.Cmdable,
	meter metrics.Meter,
	configFactory conf.IConfigLoaderFactory,
	llmClient llmruntimeservice.Client,
	fileClient fileservice.Client,
) (execute.PromptExecuteService, error) {
	wire.Build(executeSet)
	return nil, nil
}

func InitPromptOpenAPIApplication(
	idgen idgen.IIDGenerator,
	db db.Provider,
	redisCli redis.Cmdable,
	meter metrics.Meter,
	configFactory conf.IConfigLoaderFactory,
	limiterFactory limiter.IRateLimiterFactory,
	llmClient llmruntimeservice.Client,
	authClient authservice.Client,
	fileClient fileservice.Client,
	userClient userservice.Client,
) (openapi.PromptOpenAPIService, error) {
	wire.Build(openAPISet)
	return nil, nil
}
