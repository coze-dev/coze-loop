// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable security/detect-non-literal-regexp */
/* eslint-disable @coze-arch/max-line-per-function */
import { useShallow } from 'zustand/react/shallow';
import { nanoid } from 'nanoid';
import { CollapseCard } from '@cozeloop/components';
import { useModalData } from '@cozeloop/base-hooks';
import {
  ContentType,
  Role,
  TemplateType,
  type VariableDef,
  VariableType,
  type VariableVal,
} from '@cozeloop/api-schema/prompt';
import { IconCozPlus } from '@coze-arch/coze-design/icons';
import { Button, Typography } from '@coze-arch/coze-design';

import { usePromptStore } from '@/store/use-prompt-store';
import { useBasicStore } from '@/store/use-basic-store';
import { useCompare } from '@/hooks/use-compare';
import { VARIABLE_TYPE_ARRAY_TAG } from '@/consts';

import { VariableInput } from '../variable-input';
import { VariableModal } from './variable-modal';

interface VariablesCardProps {
  uid?: number;
  defaultVisible?: boolean;
}

export function VariablesCard({ uid, defaultVisible }: VariablesCardProps) {
  const {
    streaming,
    variables,
    setMessageList,
    mockVariables,
    setMockVariables,
    setVariables,
  } = useCompare(uid);
  const { readonly: basicReadonly } = useBasicStore(
    useShallow(state => ({
      readonly: state.readonly,
    })),
  );
  const { templateType } = usePromptStore(
    useShallow(state => ({
      templateType: state.templateType,
    })),
  );

  const isNormalTemplate = templateType === TemplateType.Normal;

  const readonly = basicReadonly || streaming;

  const varibaleModal = useModalData<VariableVal>();

  const onDeleteVariable = (key?: string) => {
    if (key) {
      const variableItem = variables?.find(it => it.key === key);
      if (variableItem?.type === VariableType.Placeholder) {
        setMessageList(list => {
          if (!Array.isArray(list)) {
            return [];
          }
          const newList = list?.filter(
            it => !(it.role === Role.Placeholder && it.content === key),
          );
          return newList;
        });
      }

      if (isNormalTemplate) {
        const reg = new RegExp(`{{${key}}}`, 'g');

        setMessageList(list => {
          if (!Array.isArray(list)) {
            return [];
          }
          const newList = list?.map(it => {
            if (it.content) {
              const hasReg = reg.test(it.content);
              return {
                ...it,
                key: hasReg ? nanoid() : it.key,
                content: it.content.replace(reg, ''),
              };
            } else if (it.parts?.length) {
              let needNewKey = false;
              const newParts = it.parts
                .filter(
                  part =>
                    !(
                      part.type === ContentType.MultiPartVariable &&
                      part.text === key
                    ),
                )
                .map(part => {
                  if (
                    part.type === ContentType.Text &&
                    part.text &&
                    reg.test(part.text)
                  ) {
                    needNewKey = true;
                    return {
                      ...part,
                      text: part.text.replace(reg, ''),
                    };
                  }
                  return part;
                });
              return {
                ...it,
                key:
                  needNewKey || newParts.length !== it.parts.length
                    ? nanoid()
                    : it.key,
                parts: newParts,
                content:
                  newParts.length === 1
                    ? newParts.map(part => part.text).join('')
                    : undefined,
              };
            }
            return it;
          });
          return newList;
        });
      } else {
        setVariables(list => {
          if (!Array.isArray(list)) {
            return [];
          }
          return list?.filter(it => it.key !== key);
        });
        setMockVariables(list => {
          if (!Array.isArray(list)) {
            return [];
          }
          return list?.filter(it => it.key !== key);
        });
      }
    }
  };

  const changeInputVariableValue = ({
    key,
    value,
    placeholder_messages,
    multi_part_values,
  }: VariableVal) => {
    setMockVariables(list => {
      if (!Array.isArray(list)) {
        return [];
      }
      const newList = list?.map(it => {
        if (it.key === key) {
          return {
            ...it,
            value,
            placeholder_messages,
            multi_part_values,
          };
        }
        return it;
      });
      return newList;
    });
  };

  const handleVariableModalOk = (
    def: VariableDef,
    value: VariableVal,
    isEdit?: boolean,
  ) => {
    if (isEdit) {
      setVariables(list => {
        if (!Array.isArray(list)) {
          return [def];
        }
        const newList = list?.map(it => (it.key === def.key ? def : it));
        return newList;
      });
      setMockVariables(list => {
        if (!Array.isArray(list)) {
          return [value];
        }
        const newList = list?.map(it => (it.key === def.key ? value : it));
        return newList;
      });
    } else {
      setVariables(list => {
        if (!Array.isArray(list)) {
          return [def];
        }
        return [...list, def];
      });
      setMockVariables(list => {
        if (!Array.isArray(list)) {
          return [value];
        }
        return [...list, value];
      });
    }
    varibaleModal.close();
  };

  const tagKeys = Object.values(VARIABLE_TYPE_ARRAY_TAG);

  return (
    <CollapseCard
      title={<Typography.Text strong>Prompt 变量</Typography.Text>}
      defaultVisible={defaultVisible}
    >
      <div className="flex flex-col gap-2 pt-2 pb-3">
        {mockVariables?.map(item => {
          const variable = variables?.find(it => it.key === item.key);
          const isNormalTag =
            !variable?.type_tags?.length ||
            tagKeys.includes(variable?.type_tags?.[0] ?? '');

          return (
            <VariableInput
              key={`${item.key}`}
              variableType={variable?.type}
              readonly={readonly || !isNormalTag}
              variableVal={item}
              onDelete={key => onDeleteVariable(key)}
              onValueChange={value => changeInputVariableValue({ ...value })}
              onVariableChange={value =>
                varibaleModal.open({ ...value, ...item })
              }
            />
          );
        })}

        {variables?.some(it => it.key) ||
        templateType === TemplateType.Jinja2 ? null : (
          <Typography.Text
            type="tertiary"
            style={{ color: 'var(--coz-fg-dim)' }}
          >
            暂无变量
          </Typography.Text>
        )}
        {templateType === TemplateType.Jinja2 ? (
          <Button
            color="primary"
            icon={<IconCozPlus />}
            onClick={e => {
              varibaleModal.open();
              e.stopPropagation();
            }}
            disabled={readonly}
          >
            新增变量
          </Button>
        ) : null}
      </div>
      <VariableModal
        visible={varibaleModal.visible}
        data={varibaleModal.data}
        variableList={variables}
        onCancel={varibaleModal.close}
        onOk={handleVariableModalOk}
        typeDisabled={readonly}
      />
    </CollapseCard>
  );
}
