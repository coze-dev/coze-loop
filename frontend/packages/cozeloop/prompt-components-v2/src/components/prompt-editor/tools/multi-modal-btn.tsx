import { useRef, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { TooltipWhenDisabled } from '@cozeloop/components';
import {
  type ButtonProps,
  Form,
  type FormApi,
  FormTextArea,
  Icon,
  IconButton,
  Popconfirm,
  Typography,
} from '@coze-arch/coze-design';

import { VARIABLE_MAX_LEN } from '@/consts';
import { type VariableType } from '@/components/basic-prompt-editor/extensions/variable';
import { ReactComponent as ModalVariableIcon } from '@/assets/modal-variable.svg';

interface MultiModalBtnProps {
  modalVariableEnable?: boolean;
  variables?: VariableType[];
  disabled?: boolean;
  afterInsert?: (content: string) => void;
}

export function MultiModalBtn({
  modalVariableEnable,
  variables,
  disabled,
  afterInsert,
}: MultiModalBtnProps) {
  const [modalVariableVisible, setModalVariableVisible] = useState(false);
  const [modalVariableCanAdd, setModalVariableCanAdd] = useState(false);
  const formApiRef = useRef<FormApi>();

  const handleModalVariableConfirm = () => {
    const value = formApiRef.current?.getValues();
    if (value?.content) {
      setModalVariableVisible(false);
      setModalVariableCanAdd(false);
      const content = `<multimodal-variable>${value?.content}</multimodal-variable>`;
      afterInsert?.(content);
    }
  };

  return (
    <TooltipWhenDisabled
      disabled={!modalVariableVisible}
      content={
        modalVariableEnable ? (
          <div className="flex flex-col">
            <Typography.Text className="text-white">
              {I18n.t('prompt_add_new_multi_modal_variable')}
            </Typography.Text>
            <Typography.Text
              style={{ color: 'rgba(227, 232, 250, 0.46)' }}
              type="secondary"
              size="small"
            >
              {I18n.t('prompt_support_multi_modal_in_prompt_via_variable')}
            </Typography.Text>
          </div>
        ) : (
          I18n.t('selected_model_not_support_multi_modal')
        )
      }
      theme="dark"
    >
      <span>
        <Popconfirm
          className="w-[300px]"
          title={I18n.t('prompt_add_multi_modal_variable')}
          content={
            <Form
              getFormApi={formApi => (formApiRef.current = formApi)}
              showValidateIcon={false}
              onValueChange={values => {
                setTimeout(() => {
                  const error = formApiRef.current?.getError('content');
                  if (values?.content && !error) {
                    setModalVariableCanAdd(true);
                  } else {
                    setModalVariableCanAdd(false);
                  }
                }, 100);
              }}
            >
              <FormTextArea
                noLabel
                field="content"
                placeholder={I18n.t('prompt_input_multi_modal_variable_name')}
                maxCount={50}
                maxLength={50}
                rules={[
                  {
                    validator: (_rules, value, callback) => {
                      const regex = new RegExp(
                        `^[a-zA-Z][\\w]{0,${VARIABLE_MAX_LEN - 1}}$`,
                        'gm',
                      );

                      if (value) {
                        // 检查是否包含换行符
                        if (value.includes('\n') || value.includes('\r')) {
                          callback(
                            I18n.t(
                              'prompt_variable_name_rule_letters_numbers_underscore',
                            ),
                          );
                          return false;
                        }
                        if (regex.test(value)) {
                          if (variables?.some(v => v.key === value)) {
                            callback(I18n.t('prompt_variable_name_duplicate'));
                            return false;
                          }
                          return true;
                        } else {
                          callback(
                            I18n.t(
                              'prompt_variable_name_rule_letters_numbers_underscore',
                            ),
                          );
                          return false;
                        }
                      }
                      return true;
                    },
                  },
                ]}
                rows={2}
                showCounter
                fieldClassName="!p-0"
                autoFocus
              />
            </Form>
          }
          okText={I18n.t('confirm')}
          okButtonProps={
            {
              disabled: !modalVariableCanAdd,
              'data-btm': 'd99713',
              'data-btm-title': I18n.t('prompt_add_new_multi_modal_variable'),
            } as unknown as ButtonProps
          }
          stopPropagation
          trigger="custom"
          visible={modalVariableVisible}
          onConfirm={handleModalVariableConfirm}
          onClickOutSide={() => {
            setModalVariableVisible(false);
            setModalVariableCanAdd(false);
          }}
        >
          <IconButton
            color={modalVariableVisible ? 'highlight' : 'secondary'}
            size="mini"
            icon={<Icon svg={<ModalVariableIcon fontSize={12} />} />}
            onClick={() => setModalVariableVisible(v => !v)}
            disabled={!modalVariableEnable || disabled}
            data-btm="d99713"
            data-btm-title={I18n.t('prompt_add_new_multi_modal_variable')}
          />
        </Popconfirm>
      </span>
    </TooltipWhenDisabled>
  );
}
