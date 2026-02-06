// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"github.com/google/wire"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	mtr "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	evaluatormtr "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/metrics/evaluator"
	rmqproducer "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket/producer"
	evaluatorrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	targetrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/data"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/llm"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/runtime"
	evalconf "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf"
)

// ExperimentDomainServiceSet 提供所有 Experiment 相关的 Domain Service
var ExperimentDomainServiceSet = wire.NewSet(
	NewExptManager,
	NewExptResultService,
	NewExptAggrResultService,
	NewExptSchedulerSvc,
	NewExptRecordEvalService,
	NewExptAnnotateService,
	NewExptResultExportService,
	NewInsightAnalysisService,
	NewSchedulerModeFactory,
	NewExptTemplateManager,
	NewEvaluationAnalysisService,
	// Repo Sets
	experimentrepo.ExperimentRepoSet,
)

// EvaluatorDomainServiceSet 提供所有 Evaluator 相关的 Domain Service
var EvaluatorDomainServiceSet = wire.NewSet(
	NewEvaluatorServiceImpl,
	NewEvaluatorRecordServiceImpl,
	NewEvaluatorTemplateService,
	NewEvaluatorSourceServices,
	NewCodeBuilderFactory,
	evalconf.NewEvaluatorConfiger,
	// Infrastructure Sets
	llm.LLMRPCSet,
	runtime.RuntimeSet,
	evaluatormtr.EvaluatorMetricsSet,
	rmqproducer.MQProducerSet,
	// Repo Sets
	evaluatorrepo.EvaluatorRepoSet,
)

// EvaluationSetDomainServiceSet 提供所有 EvaluationSet 相关的 Domain Service
var EvaluationSetDomainServiceSet = wire.NewSet(
	NewEvaluationSetVersionServiceImpl,
	NewEvaluationSetItemServiceImpl,
	NewEvaluationSetServiceImpl,
	NewEvaluationSetSchemaServiceImpl,
	// Infrastructure Sets
	data.DataRPCSet,
)

// TargetDomainServiceSet 提供所有 Target 相关的 Domain Service
var TargetDomainServiceSet = wire.NewSet(
	NewEvalTargetServiceImpl,
	NewSourceTargetOperators,
	// Infrastructure Sets
	prompt.PromptRPCSet,
	// Repo Sets
	targetrepo.TargetRepoSet,
)

// NewEvaluatorSourceServices 创建评估器源服务映射
func NewEvaluatorSourceServices(
	llmProvider rpc.ILLMProvider,
	metric mtr.EvaluatorExecMetrics,
	config evalconf.IConfiger,
	runtimeManager component.IRuntimeManager,
	codeBuilderFactory CodeBuilderFactory,
) map[entity.EvaluatorType]EvaluatorSourceService {
	// 设置codeBuilderFactory的runtimeManager依赖
	codeBuilderFactory.SetRuntimeManager(runtimeManager)

	services := []EvaluatorSourceService{
		NewEvaluatorSourcePromptServiceImpl(llmProvider, metric, config),
		NewEvaluatorSourceCodeServiceImpl(runtimeManager, codeBuilderFactory, metric),
	}

	serviceMap := make(map[entity.EvaluatorType]EvaluatorSourceService)
	for _, svc := range services {
		serviceMap[svc.EvaluatorType()] = svc
	}
	return serviceMap
}

// NewSourceTargetOperators 创建源目标操作器映射
func NewSourceTargetOperators(adapter rpc.IPromptRPCAdapter) map[entity.EvalTargetType]ISourceEvalTargetOperateService {
	return map[entity.EvalTargetType]ISourceEvalTargetOperateService{
		entity.EvalTargetTypeLoopPrompt: NewPromptSourceEvalTargetServiceImpl(adapter),
	}
}
