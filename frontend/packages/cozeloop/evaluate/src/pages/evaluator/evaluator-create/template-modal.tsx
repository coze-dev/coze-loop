/* eslint-disable @coze-arch/max-line-per-function */
import { useEffect, useState } from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { TemplateInfo } from '@cozeloop/evaluate-components';
import {
  TemplateType,
  type EvaluatorContent,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozCrossFill } from '@coze-arch/coze-design/icons';
import { Button, IconButton, Modal, Spin } from '@coze-arch/coze-design';

import styles from './template-modal.module.less';

export function TemplateModal({
  visible,
  disabled,
  onCancel,
  onSelect,
}: {
  visible: boolean;
  disabled?: boolean;
  onCancel: () => void;
  onSelect: (template: EvaluatorContent) => void;
}) {
  const [selected, setSelected] = useState<EvaluatorContent>();
  const [keyMap, setKeyMap] = useState<Record<string, EvaluatorContent>>({});

  const currentData =
    selected && keyMap[selected.prompt_evaluator?.prompt_template_key || ''];

  const listService = useRequest(
    async () =>
      StoneEvaluationApi.ListTemplates({
        builtin_template_type: TemplateType.Prompt,
      }),
    {
      manual: true,
      onSuccess: data => {
        const firstItem = data?.builtin_template_keys?.[0];
        if (firstItem) {
          setSelected(firstItem);
        }
      },
    },
  );

  const detailService = useRequest(
    async () => {
      const key = selected?.prompt_evaluator?.prompt_template_key;
      if (key && !keyMap[key]) {
        const res = await StoneEvaluationApi.GetTemplateInfo({
          builtin_template_key: key,
          builtin_template_type: TemplateType.Prompt,
        });

        if (res.builtin_template) {
          setKeyMap({
            ...keyMap,
            [key]: res.builtin_template,
          });
        }
      }
    },
    {
      ready: Boolean(selected),
      refreshDeps: [selected],
    },
  );

  useEffect(() => {
    if (visible && !listService.data) {
      listService.run();
    }
  }, [visible]);

  return (
    <Modal
      className={styles.modal}
      width={1040}
      height="fill"
      visible={visible}
      header={null}
      footer={null}
    >
      <div
        className="overflow-hidden w-full flex flex-row"
        style={{
          height: 'calc(100% + 32px)',
        }}
      >
        <div className="coz-bg-primary w-60 flex flex-col">
          <div className="m-4 pl-2 h-10 flex items-center text-[20px] coz-fg-plus font-medium">
            {I18n.t('select_template')}
          </div>
          <div className="p-4 pt-0 overflow-y-auto styled-scrollbar pr-[10px]">
            {listService.loading ? (
              <Spin
                spinning={true}
                style={{
                  width: '100%',
                }}
              />
            ) : (
              <>
                <div className="p-2 text-sm leading-4 font-medium coz-fg-secondary mb-1">
                  {I18n.t('preset_evaluator')}
                </div>
                {listService.data?.builtin_template_keys?.map((t, idx) => (
                  <div
                    key={idx}
                    className={classNames(
                      'p-2 text-sm leading-4 font-medium coz-fg-primary rounded-[6px] mb-1 cursor-pointer',
                      selected === t
                        ? 'bg-[#ABB5FF4D]'
                        : 'hover:coz-mg-primary',
                    )}
                    onClick={() => {
                      setSelected(t);
                    }}
                  >
                    {t.prompt_evaluator?.prompt_template_name}
                  </div>
                ))}
              </>
            )}
          </div>
        </div>
        <div className="w-0 flex-1 flex flex-col">
          <div className="flex-shrink-0 mx-6 my-4 h-10 flex items-center justify-between text-[20px] coz-fg-plus font-medium">
            {I18n.t('preview')}
            <IconButton
              size="small"
              icon={<IconCozCrossFill className="!w-4 !h-4 coz-fg-secondary" />}
              className="!max-w-[24px] !w-6 !h-6 !p-1"
              color="secondary"
              onClick={onCancel}
            />
          </div>
          <div className="flex-1 px-6 pb-4 pt-0 overflow-y-auto styled-scrollbar pr-[18px]">
            {listService.loading || detailService.loading ? (
              <Spin
                spinning={true}
                style={{
                  width: '100%',
                }}
              />
            ) : (
              <TemplateInfo data={currentData} />
            )}
          </div>
          <div className="flex flex-row justify-end gap-2 px-6 pt-2 pb-6">
            <Button color="primary" onClick={onCancel}>
              {I18n.t('cancel')}
            </Button>
            <Button
              color="brand"
              disabled={!currentData || disabled}
              onClick={() => currentData && onSelect(currentData)}
            >
              {I18n.t('confirm')}
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
}
