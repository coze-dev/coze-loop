// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	dataapp "github.com/coze-dev/coze-loop/backend/modules/data/application"
	dataconsumer "github.com/coze-dev/coze-loop/backend/modules/data/infra/mq/consumer"
	exptapp "github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	evalconsumer "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket/consumer"
	obapp "github.com/coze-dev/coze-loop/backend/modules/observability/application"
	obconsumer "github.com/coze-dev/coze-loop/backend/modules/observability/infra/mq/consumer"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

func mustInitConsumerWorkers(
	cfactory conf.IConfigLoaderFactory,
	experimentApplication exptapp.IExperimentApplication,
	datasetApplication dataapp.IJobRunMsgHandler,
	obApplication obapp.IObservabilityOpenAPIApplication,
) []mq.IConsumerWorker {
	var res []mq.IConsumerWorker

	workers, err := evalconsumer.NewConsumerWorkers(cfactory, experimentApplication)
	if err != nil {
		panic(err)
	}
	res = append(res, workers...)

	workers, err = dataconsumer.NewConsumerWorkers(cfactory, datasetApplication)
	if err != nil {
		panic(err)
	}
	res = append(res, workers...)

	loader, err := cfactory.NewConfigLoader("observability.yaml")
	if err != nil {
		panic(err)
	}
	workers, err = obconsumer.NewConsumerWorkers(loader, obApplication)
	if err != nil {
		panic(err)
	}
	res = append(res, workers...)

	return res
}
