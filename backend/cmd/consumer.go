// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	dataapp "github.com/coze-dev/coze-loop/backend/modules/data/application"
	dataconsumer "github.com/coze-dev/coze-loop/backend/modules/data/infra/mq/consumer"
	exptapp "github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	domainservice "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	evalconsumer "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket/consumer"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket/producer"
	obapp "github.com/coze-dev/coze-loop/backend/modules/observability/application"
	obconsumer "github.com/coze-dev/coze-loop/backend/modules/observability/infra/mq/consumer"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

func MustInitConsumerWorkers(
	cfactory conf.IConfigLoaderFactory,
	mqFactory mq.IFactory,
	experimentApplication exptapp.IExperimentApplication,
	datasetApplication dataapp.IJobRunMsgHandler,
	obApplication obapp.IObservabilityOpenAPIApplication,
	taskApplication obapp.ITaskApplication,
) []mq.IConsumerWorker {
	var res []mq.IConsumerWorker

	loader, err := cfactory.NewConfigLoader(consts.EvaluationConfigFileName)
	if err != nil {
		panic(err)
	}

	publisher, err := producer.NewExptEventPublisher(context.Background(), cfactory, mqFactory)
	if err != nil {
		panic(err)
	}

	var settlement []domainservice.CentralItemSettlementDeps
	if provider, ok := experimentApplication.(interface {
		CentralSettlementDeps() (domainservice.CentralItemSettlementDeps, bool)
	}); ok {
		if deps, ready := provider.CentralSettlementDeps(); ready {
			settlement = append(settlement, deps)
		}
	}

	workers, err := evalconsumer.NewConsumerWorkers(loader, experimentApplication, publisher, settlement...)
	if err != nil {
		panic(err)
	}
	res = append(res, workers...)

	workers, err = dataconsumer.NewConsumerWorkers(cfactory, datasetApplication)
	if err != nil {
		panic(err)
	}
	res = append(res, workers...)

	loader, err = cfactory.NewConfigLoader("observability.yaml")
	if err != nil {
		panic(err)
	}
	workers, err = obconsumer.NewConsumerWorkers(loader, obApplication, taskApplication)
	if err != nil {
		panic(err)
	}
	res = append(res, workers...)

	return res
}
