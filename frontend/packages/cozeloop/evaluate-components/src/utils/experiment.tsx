import { get } from 'lodash-es';
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
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
    warnText =
      '仅评测集相同且已执行完成的实验可进行对比。目前选择的实验有关联评测集不同的情况，请重新选择。';
  } else if (experiments.length > MAX_EXPERIMENT_CONTRAST_COUNT) {
    warnText = `实验对比最大数量不能超过 ${MAX_EXPERIMENT_CONTRAST_COUNT} 个，请重新选择。`;
  } else if (!checkExperimentsStatus(experiments)) {
    warnText = '仅已执行完成的实验可进行对比，请重新选择。';
  }
  if (!warnText) {
    return true;
  }

  sendEvent(EVENT_NAMES.cozeloop_experiment_compare, {
    from: 'experiment_compare_fail_modal',
  });

  Modal.info({
    title: '实验对比发起失败',
    content: <div className="mt-2">{warnText}</div>,
    okText: '已知晓',
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
  return `${index <= 0 ? '基准组' : `实验组${index}`}${showName ? ` - ${experiment?.name}` : ''}`;
}
