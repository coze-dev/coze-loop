// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { AggregatorType } from '@cozeloop/api-schema/evaluation';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';
import { Button, Select } from '@coze-arch/coze-design';

import styles from './index.module.less';

const aggregatorTypeOptions = [
  {
    label: 'Avg',
    value: AggregatorType.Average,
  },
  {
    label: 'Max',
    value: AggregatorType.Max,
  },
  {
    label: 'Min',
    value: AggregatorType.Min,
  },
  {
    label: 'Sum',
    value: AggregatorType.Sum,
  },
];

const valueToLabelMap: Record<AggregatorType, string> = {
  [AggregatorType.Average]: 'Avg',
  [AggregatorType.Max]: 'Max',
  [AggregatorType.Min]: 'Min',
  [AggregatorType.Sum]: 'Sum',
  [AggregatorType.Distribution]: 'Distribution',
};

export function ExperimentScoreTypeSelect({
  value,
  onChange,
}: {
  value?: AggregatorType;
  onChange?: (value: AggregatorType) => void;
}) {
  return (
    <Select
      value={value}
      className={styles['small-radio-select']}
      onChange={val => onChange?.(val as AggregatorType)}
      optionList={aggregatorTypeOptions}
      triggerRender={() => (
        <Button color="primary" size="small">
          <div className="flex items-center gap-x-1">
            <span>{valueToLabelMap[value || AggregatorType.Average]}</span>
            <IconCozArrowDown />
          </div>
        </Button>
      )}
    />
  );
}
