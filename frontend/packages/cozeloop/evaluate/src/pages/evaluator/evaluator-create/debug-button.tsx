import { type RefObject, useState } from 'react';

import { cloneDeep } from 'lodash-es';
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
        {'调试'}
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
