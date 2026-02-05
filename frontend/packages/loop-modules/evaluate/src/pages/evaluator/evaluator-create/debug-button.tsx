// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type RefObject, useState } from 'react';

import { cloneDeep } from 'lodash-es';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';
import { IconCozPlayFill } from '@coze-arch/coze-design/icons';
import { Button, type Form } from '@coze-arch/coze-design';

import { DebugModal } from './debug-modal';

export interface DebugButtonProps {
  formApi?: RefObject<Form<Evaluator>>;
  onApplyValue?: () => void;
}

export function DebugButton({ formApi, onApplyValue }: DebugButtonProps) {
  const [debugValue, setDebugValue] = useState<Evaluator>();

  return (
    <>
      <Button
        icon={<IconCozPlayFill />}
        color="highlight"
        onClick={() =>
          setDebugValue(formApi?.current?.formApi?.getValues() || {})
        }
      >
        {I18n.t('debug')}
      </Button>
      {debugValue ? (
        <DebugModal
          initValue={debugValue}
          onCancel={() => setDebugValue(undefined)}
          onSubmit={(newValue: Evaluator) => {
            const saveData = cloneDeep(newValue);
            formApi?.current?.formApi?.setValues(saveData, {
              isOverride: true,
            });
            setDebugValue(undefined);
            onApplyValue?.();
          }}
        />
      ) : null}
    </>
  );
}
