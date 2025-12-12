/* eslint-disable @coze-arch/max-line-per-function */
import { useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { TemplateType } from '@cozeloop/api-schema/prompt';
import {
  IconCozArrowDown,
  IconCozQuestionMarkCircle,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  type ButtonColor,
  Modal,
  Popover,
  Tooltip,
  Typography,
} from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { LABEL_MAP } from '@/consts';

interface TemplateSelectProps {
  streaming?: boolean;
  color?: ButtonColor;
}

export function TemplateSelect({ streaming, color }: TemplateSelectProps) {
  const [dropVisible, setDropVisible] = useState(false);

  const { templateType, setTemplateType } = usePromptStore(
    useShallow(state => ({
      templateType: state.templateType,
      setTemplateType: state.setTemplateType,
    })),
  );

  const templateTypeChange = (type: TemplateType) => {
    if (templateType?.type === type) {
      return;
    }

    setDropVisible(false);
    Modal.confirm({
      title: I18n.t('prompt_switch_template_engine'),
      content: I18n.t('prompt_may_cause_variable_render_failure'),
      onOk: () => {
        setTemplateType({ type, value: type });
      },
      okText: I18n.t('global_btn_confirm'),
      okButtonColor: 'yellow',
      cancelText: I18n.t('cancel'),
    });
  };

  return (
    <div className="flex items-center gap-3">
      <Popover
        trigger="custom"
        visible={dropVisible}
        content={
          <div className="px-4 pt-3 pb-4 w-[350px]">
            <Typography.Text strong className="mb-3 block">
              {I18n.t('evaluate_select_template')}
            </Typography.Text>

            <div className="flex flex-col gap-2">
              <div
                className={classNames(
                  '!h-fit !px-3 !pt-1.5 !pb-3 border border-solid coz-stroke-primary rounded-lg cursor-pointer hover:bg-[#969fff26]',
                  {
                    'coz-stroke-hglt':
                      templateType?.type === TemplateType.Normal,
                    'bg-[#969fff26]':
                      templateType?.type === TemplateType.Normal,
                  },
                )}
                onClick={() => {
                  templateTypeChange(TemplateType.Normal);
                }}
              >
                <div className="flex flex-col items-start">
                  <Typography.Text strong style={{ lineHeight: '32px' }}>
                    {I18n.t('prompt_normal_template_engine')}
                  </Typography.Text>
                  <Typography.Text
                    size="small"
                    className="!text-[13px] !leading-[20px] !coz-fg-secondary"
                  >
                    {I18n.t('prompt_triple_braces_variable_recognition')}
                  </Typography.Text>
                </div>
              </div>

              <div
                className={classNames(
                  'items-start !h-fit !px-3 !pt-1.5 !pb-3 border border-solid coz-stroke-primary rounded-lg cursor-pointer hover:bg-[#969fff26]',
                  {
                    'coz-stroke-hglt':
                      templateType?.type === TemplateType.Jinja2,
                    'bg-[#969fff26]':
                      templateType?.type === TemplateType.Jinja2,
                  },
                )}
                onClick={() => {
                  templateTypeChange(TemplateType.Jinja2);
                }}
              >
                <div className="flex flex-col items-start">
                  <Typography.Text strong style={{ lineHeight: '32px' }}>
                    {I18n.t('prompt_jinja2_template_engine')}
                  </Typography.Text>
                  <Typography.Text
                    size="small"
                    className="!text-[13px] !leading-[20px] !coz-fg-secondary flex items-center gap-1"
                  >
                    {I18n.t('prompt_manual_add_delete_variables_complex_logic')}
                    <Tooltip
                      content={
                        <>
                          {I18n.t('view')}
                          <a
                            href="https://loop.coze.cn/open/docs/cozeloop/create-prompt#51f641db"
                            target="_blank"
                            style={{
                              color: '#AAA6FF',
                              textDecoration: 'none',
                            }}
                          >
                            {I18n.t('prompt_user_manual')}
                          </a>
                        </>
                      }
                      stopPropagation
                      theme="dark"
                    >
                      <IconCozQuestionMarkCircle />
                    </Tooltip>
                  </Typography.Text>
                </div>
              </div>

              <div
                className={classNames(
                  'items-start !h-fit !px-3 !pt-1.5 !pb-3 border border-solid coz-stroke-primary rounded-lg cursor-pointer hover:bg-[#969fff26]',
                  {
                    'coz-stroke-hglt':
                      templateType?.type === TemplateType.GoTemplate,
                    'bg-[#969fff26]':
                      templateType?.type === TemplateType.GoTemplate,
                  },
                )}
                onClick={() => {
                  templateTypeChange(TemplateType.GoTemplate);
                }}
              >
                <div className="flex flex-col items-start">
                  <Typography.Text strong style={{ lineHeight: '32px' }}>
                    {I18n.t('prompt_gotemplate_engine')}
                  </Typography.Text>
                  <Typography.Text
                    size="small"
                    className="!text-[13px] !leading-[20px] !coz-fg-secondary flex items-center gap-1"
                  >
                    {I18n.t('prompt_manual_add_delete_variables_complex_logic')}
                    <Tooltip
                      content={
                        <>
                          {I18n.t('view')}
                          <a
                            href="https://loop.coze.cn/open/docs/cozeloop/create-prompt#51f641db"
                            target="_blank"
                            style={{
                              color: '#AAA6FF',
                              textDecoration: 'none',
                            }}
                          >
                            {I18n.t('prompt_user_manual')}
                          </a>
                        </>
                      }
                      stopPropagation
                      theme="dark"
                    >
                      <IconCozQuestionMarkCircle />
                    </Tooltip>
                  </Typography.Text>
                </div>
              </div>
            </div>
          </div>
        }
        position="topLeft"
        onClickOutSide={() => setDropVisible(false)}
      >
        <Button
          icon={<IconCozArrowDown />}
          iconPosition="right"
          color={color ?? 'secondary'}
          className={classNames('!font-medium', {
            '!border border-solid coz-stroke-primary ': !color,
          })}
          onClick={() => !streaming && setDropVisible(true)}
          disabled={streaming}
          size="mini"
        >
          {LABEL_MAP[templateType?.type || TemplateType.Normal]}
        </Button>
      </Popover>
    </div>
  );
}
