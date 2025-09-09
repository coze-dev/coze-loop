// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { ContentType, DatasetItem } from '@cozeloop/evaluate-components';
import { type VariableDef } from '@cozeloop/api-schema/prompt';
import { Collapse } from '@coze-arch/coze-design';

import styles from './index.module.less';

export function MultiPartEdit(props: {
  variable: VariableDef | undefined;
  value?: unknown;
  onChange?: (value: unknown) => void;
}) {
  const { key } = props.variable ?? {};
  return (
    <Collapse
      defaultActiveKey={'1'}
      expandIconPosition="right"
      keepDOM={true}
      className={styles.collapse}
    >
      <Collapse.Panel
        header={<span className="text-[12px]">{key}</span>}
        itemKey="1"
      >
        <DatasetItem
          fieldSchema={{
            content_type: ContentType.MultiPart,
          }}
          fieldContent={{
            content_type: ContentType.MultiPart,
            ...(props.value || {}),
          }}
          isEdit={true}
          className="bg-inherit"
          onChange={val => {
            props.onChange?.({
              content_type: ContentType.MultiPart,
              multi_part: val?.multi_part,
            });
          }}
        />
      </Collapse.Panel>
    </Collapse>
  );
}
