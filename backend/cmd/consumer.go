// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	dataapp "github.com/coze-dev/coze-loop/backend/modules/data/application"
	dataconsumer "github.com/coze-dev/coze-loop/backend/modules/data/infra/mq/consumer"
	exptapp "github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	evalconsumer "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket/consumer"
	obapp "github.com/coze-dev/coze-loop/backend/modules/observability/application"
	obconsumer "github.com/coze-dev/coze-loop/backend/modules/observability/infra/mq/consumer"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

func MustInitConsumerWorkers(
	cfactory conf.IConfigLoaderFactory,
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
	workers, err := evalconsumer.NewConsumerWorkers(loader, experimentApplication)
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
