// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef } from 'react';

import { useRequest } from 'ahooks';
import { type Label } from '@cozeloop/api-schema/prompt';
import { StonePromptApi } from '@cozeloop/api-schema';
import { Button, Form, type FormApi, Modal } from '@coze-arch/coze-design';

import { VersionLabelTitle } from './version-label-title';
import {
  checkLabelDuplicate,
  FormVersionLabelSelect,
  type LabelWithPromptVersion,
} from './version-label-select';

interface Props {
  visible: boolean;
  spaceID: string;
  promptID: string;
  version?: string;
  labels: Label[];
  onCancel?: () => void;
  onConfirm?: (labels: string[]) => void;
}
export function VersionLabelModal(props: Props) {
  const formApiRef = useRef<FormApi<{ labels: LabelWithPromptVersion[] }>>();
  const service = useRequest(
    async () => {
      const values = formApiRef.current?.getValues();
      await StonePromptApi.UpdateCommitLabels({
        workspace_id: props.spaceID,
        prompt_id: props.promptID,
        commit_version: props.version,
        label_keys: values?.labels.map(item => item.key),
      });
      return values?.labels || [];
    },
    {
      manual: true,
      onSuccess: val => {
        props.onConfirm?.(val.map(item => item.key));
      },
    },
  );

  return (
    <Modal
      title="修改版本标识"
      visible={props.visible}
      onCancel={props.onCancel}
      width={640}
      footer={
        <div>
          <Button
            color="primary"
            onClick={e => {
              e.stopPropagation();
              props.onCancel?.();
            }}
          >
            取消
          </Button>
          <Button
            onClick={async e => {
              e.stopPropagation();
              const values = formApiRef.current?.getValues();
              await checkLabelDuplicate(values?.labels, props.version);
              service.run();
            }}
            loading={service.loading}
          >
            提交
          </Button>
        </div>
      }
    >
      <Form
        getFormApi={formApi => (formApiRef.current = formApi)}
        initValues={{
          labels: props.labels || [],
        }}
        onClick={e => e.stopPropagation()}
      >
        <FormVersionLabelSelect
          promptID={props.promptID}
          label={<VersionLabelTitle />}
          field="labels"
        />
      </Form>
    </Modal>
  );
}
