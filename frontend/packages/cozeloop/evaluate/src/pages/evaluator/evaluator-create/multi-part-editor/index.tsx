import {
  ContentType,
  DatasetItem,
  type PromptVariable,
} from '@cozeloop/evaluate-components';
import { Collapse } from '@coze-arch/coze-design';

import styles from './index.module.less';

export function MultiPartEdit(props: {
  variable: PromptVariable | undefined;
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
