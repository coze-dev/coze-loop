import { useEffect, useRef } from 'react';

import { Guard, GuardPoint } from '@cozeloop/guard';
import { IconCozPlayFill, IconCozMinimize } from '@coze-arch/coze-design/icons';
import { Form, Modal, Button } from '@coze-arch/coze-design';

import type { IFormValues } from '@/components/evaluator-code/types';
import CodeEvaluatorConfig from '@/components/evaluator-code';

import styles from './index.module.less';

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
            评估器配置
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
                试运行
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
