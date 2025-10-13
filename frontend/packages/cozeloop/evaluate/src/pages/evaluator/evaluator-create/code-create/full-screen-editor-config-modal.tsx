/*
 * Copyright 2025 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useEffect, useRef } from 'react';

import { Guard, GuardPoint } from '@cozeloop/guard';
import { IconCozPlayFill, IconCozMinimize } from '@coze-arch/coze-design/icons';
import { Form, Modal, Button } from '@coze-arch/coze-design';

import type { IFormValues } from '@/components/evaluator-code/types';
import CodeEvaluatorConfig from '@/components/evaluator-code';

import styles from './index.module.less';
import { I18n } from '@cozeloop/i18n-adapter';

const INIT_DELAY = 200;

interface FullScreenEditorConfigWrapperProps {
  formRef?: React.RefObject<Form>;
  visible: boolean;
  debugLoading?: boolean;
  setVisible: (visible: boolean) => void;
  onInit?: (ref: React.RefObject<Form<IFormValues>>) => void;
  onRun?: (ref: React.RefObject<Form<IFormValues>>) => Promise<unknown>;
  onCancel: () => void;
  onChange: (value: IFormValues) => void;
}

const FullScreenEditorConfigModal = (
  props: FullScreenEditorConfigWrapperProps,
) => {
  const {
    visible,
    formRef,
    onRun,
    onChange,
    setVisible,
    onInit,
    debugLoading,
  } = props;
  const localFormRef = useRef<Form<IFormValues>>(null);
  const timerRef = useRef<NodeJS.Timeout | null>(null);

  useEffect(() => {
    // 弹窗初始化时，将外部表单的值设置到全屏表单
    if (visible) {
      timerRef.current = setTimeout(() => {
        if (onInit) {
          onInit(localFormRef);
        } else {
          const outerVs = formRef?.current?.formApi?.getValues();
          localFormRef.current?.formApi.setValues(outerVs);
        }
      }, INIT_DELAY);
    }
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    };
  }, [visible, formRef, onInit]);

  return (
    <Modal
      className={styles['full-screen-modal']}
      hasScroll={false}
      header={
        <div className="flex flex-row items-center justify-between h-[78px] py-4 px-6 pb-1">
          <div
            className="text-[20px] font-medium"
            style={{ color: 'rgba(8,13,30,0.90)' }}
          >
            {I18n.t('evaluate_evaluator_config')}
          </div>
          <div className="flex flex-row items-center gap-2">
            <Guard point={GuardPoint['eval.evaluator_create.debug']} realtime>
              <Button
                color="highlight"
                icon={<IconCozPlayFill />}
                onClick={async () => {
                  const result = await onRun?.(localFormRef);
                  if (result) {
                    localFormRef.current?.formApi.setValue(
                      'config.runResults',
                      result,
                    );
                  }
                }}
                loading={debugLoading}
              >
                {I18n.t('evaluate_trial_run')}
              </Button>
            </Guard>

            <Button
              color="primary"
              icon={<IconCozMinimize />}
              onClick={() => setVisible(false)}
              loading={debugLoading}
            />
          </div>
        </div>
      }
      visible={visible}
      onCancel={() => {
        if (debugLoading) {
          return;
        }
        setVisible(false);
      }}
      width="100vw"
      fullScreen={true}
      bodyStyle={{
        height: 'calc(100vh - 120px)',
        padding: '20px',
        overflow: 'hidden',
      }}
      footer={null}
      closable={true}
      maskClosable={false}
      keepDOM={false}
    >
      <Form
        className={styles['full-screen-form-wrapper']}
        ref={localFormRef}
        onValueChange={onChange}
        style={{ height: '100%' }}
      >
        <CodeEvaluatorConfig
          field="config"
          fieldPath="config"
          fieldStyle={{ height: '100%' }}
          noLabel={true}
          debugLoading={debugLoading}
        />
      </Form>
    </Modal>
  );
};

export { FullScreenEditorConfigModal };
