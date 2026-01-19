// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { get } from 'lodash-es';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  type Evaluator,
  type Experiment,
  ExptStatus,
} from '@cozeloop/api-schema/evaluation';
import { Modal } from '@coze-arch/coze-design';

import { MAX_EXPERIMENT_CONTRAST_COUNT } from '../constants/experiment';

/**
 * 提取实验列表中所有评估器并按照评估器唯一版本去重
 */
export function uniqueExperimentsEvaluators(experiments: Experiment[]) {
  const evaluators: Evaluator[] = [];

  experiments.forEach(experiment => {
    experiment.evaluators?.forEach(evaluator => {
      evaluators.push(evaluator);
    });
  });

  const evaluatorMap: Record<string, Evaluator> = {};
  evaluators.forEach(evaluator => {
    const versionId = evaluator.current_version?.id ?? '';
    if (evaluatorMap[versionId]) {
      return;
    }
    evaluatorMap[versionId] = evaluator;
  });
  return Object.values(evaluatorMap);
}

/** 校验对比实验是否合法，并报错，返回值为是否成功 */
export function verifyContrastExperiment(experiments: Experiment[]) {
  let warnText = '';
  if (!hasSameDataset(experiments)) {
    warnText = I18n.t('experiments_compared_tip');
  } else if (experiments.length > MAX_EXPERIMENT_CONTRAST_COUNT) {
    warnText = `${I18n.t('cozeloop_open_evaluate_max_experiment_contrast_limit', { MAX_EXPERIMENT_CONTRAST_COUNT })}`;
  } else if (!checkExperimentsStatus(experiments)) {
    warnText = I18n.t('only_completed_experiments_can_be_compared');
  }
  if (!warnText) {
    return true;
  }

  sendEvent(EVENT_NAMES.cozeloop_experiment_compare, {
    from: 'experiment_compare_fail_modal',
  });

  Modal.info({
    title: I18n.t('experiment_comparison_initiation_failure'),
    content: <div className="mt-2">{warnText}</div>,
    okText: I18n.t('known'),
    closable: true,
    width: 420,
  });

  function hasSameDataset(list?: Experiment[]): boolean {
    const firstDatasetId = list?.[0]?.eval_set?.id;
    if (!firstDatasetId) {
      return true;
    }
    return list.every(item => item?.eval_set?.id === firstDatasetId);
  }
  function checkExperimentsStatus(list?: Experiment[]) {
    return list?.every(
      item =>
        item.status === ExptStatus.Success ||
        item.status === ExptStatus.Failed ||
        item.status === ExptStatus.Terminated,
    );
  }
  return false;
}

export function arrayToMap<T, R = T>(
  array: T[],
  key: keyof T,
  path = '',
): Record<string, R> {
  const map = {} as unknown as Record<string, R>;

  array.forEach(item => {
    const mapKey = item[key as keyof T] as string;
    if (mapKey !== undefined) {
      const val = path ? get(item, path) : item;
      map[mapKey] = val;
    }
  });
  return map;
}

// 计算表格跨页选中的行数据
export function getTableSelectionRows(
  selectedKeys: string[],
  rows: { id?: string }[],
  originRows: { id?: string }[],
) {
  const map = arrayToMap([...rows, ...originRows], 'id');
  const newRows = selectedKeys.map(key => map[key]).filter(Boolean);
  return newRows;
}

export function getExperimentNameWithIndex(
  experiment: Experiment | undefined,
  index: number,
  showName = true,
) {
  return `${index <= 0 ? I18n.t('benchmark_group') : `${I18n.t('experimental_group')}${index}`}${showName ? ` - ${experiment?.name}` : ''}`;
}
