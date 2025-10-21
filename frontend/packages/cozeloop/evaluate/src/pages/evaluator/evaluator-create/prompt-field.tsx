/* eslint-disable max-lines-per-function */
// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
import { useLocation } from 'react-router-dom';
import { useEffect, useMemo, useState } from 'react';

import cls from 'classnames';
import { useDebounceFn } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  PromptVariablesList,
  parseMessagesVariables,
  EvaluatorPromptEditor,
  type EvaluatorPromptEditorProps,
} from '@cozeloop/evaluate-components';
import { type VariableDef, VariableType } from '@cozeloop/api-schema/prompt';
import {
  type EvaluatorContent,
  Role,
  type PromptEvaluator,
  PromptSourceType,
  type Message,
  ContentType,
  type common,
  TemplateType,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  IconCozPlus,
  IconCozTemplate,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  Form,
  Popconfirm,
  useFieldApi,
  useFieldState,
  useFormApi,
  useFormState,
  withField,
} from '@coze-arch/coze-design';

import { TemplateModal } from './template-modal';

import styles from './prompt-field.module.less';

const messageTypeList = [
  {
    label: 'System',
    value: Role.System,
  },
  {
    label: 'User',
    value: Role.User,
  },
];

export function PromptField({
  refreshEditorKey = 0,
  disabled,
  multiModalVariableEnable,
}: {
  refreshEditorKey?: number;
  disabled?: boolean;
  multiModalVariableEnable?: boolean;
}) {
  const location = useLocation();
  const [templateVisible, setTemplateVisible] = useState(false);
  const [refreshEditorKey2, setRefreshEditorKey2] = useState(0);

  const [confirmLoading, setConfirmLoading] = useState(false);

  const formApi = useFormApi();
  const { values: formValues } = useFormState();

  const promptEvaluatorFieldApi = useFieldApi(
    'current_version.evaluator_content.prompt_evaluator',
  );
  const promptEvaluatorFieldState = useFieldState(
    'current_version.evaluator_content.prompt_evaluator',
  );

  const promptEvaluator: PromptEvaluator = promptEvaluatorFieldState.value;

  const [variables, setVariables] = useState<VariableDef[]>([]);

  const calcVariables = useDebounceFn(
    () => {
      const messages = promptEvaluator?.message_list ?? [];
      const newVariables = parseMessagesVariables(messages);
      setVariables(newVariables);
    },
    { wait: 500 },
  );

  const systemMsg = useMemo(
    () => ({
      role: Role.System,
      content: promptEvaluator?.message_list?.[0]?.content,
    }),
    [promptEvaluator?.message_list?.[0]?.content],
  );
  const userMsg = useMemo(
    () => ({
      role: Role.User,
      content: promptEvaluator?.message_list?.[1]?.content,
    }),
    [promptEvaluator?.message_list?.[1]?.content],
  );

  const afterTemplateSelect = (payload: PromptEvaluator) => {
    promptEvaluatorFieldApi.setValue({
      ...promptEvaluator,
      model_config:
        formValues?.current_version?.evaluator_content?.prompt_evaluator
          ?.model_config,
      message_list: payload.message_list,
      prompt_source_type: PromptSourceType.BuiltinTemplate,
      prompt_template_key: payload.prompt_template_key,
      prompt_template_name: payload.prompt_template_name,
    });
    if (!formValues?.name) {
      formApi.setValue('name', payload.prompt_template_name);
    }
  };

  useEffect(() => {
    calcVariables.run();
  }, [promptEvaluator?.message_list]);

  // 从URL查询参数中获取模板信息
  useEffect(() => {
    const searchParams = new URLSearchParams(location.search);
    const templateKey = searchParams.get('templateKey');

    // 如果URL中存在模板键，则加载模板
    if (templateKey) {
      // 获取模板信息
      StoneEvaluationApi.GetTemplateInfo({
        builtin_template_key: templateKey,
        builtin_template_type: TemplateType.Prompt,
      }).then(res => {
        if (res.builtin_template?.prompt_evaluator) {
          afterTemplateSelect(res.builtin_template.prompt_evaluator);
          setRefreshEditorKey2(pre => pre + 1);
        }
      });
    }
  }, [location.search]);

  const handleTemplateSelect = (template?: EvaluatorContent) => {
    // 将模板信息添加到URL查询参数
    if (template?.prompt_evaluator?.prompt_template_key) {
      setConfirmLoading(true);
      const templateKey = template?.prompt_evaluator.prompt_template_key;
      const searchParams = new URLSearchParams(location.search);
      searchParams.set('templateKey', templateKey);

      if (template?.prompt_evaluator) {
        afterTemplateSelect(template?.prompt_evaluator);
      }

      // 更新URL而不导航
      window.history.replaceState(
        null,
        '',
        `${location.pathname}?${searchParams.toString()}`,
      );
    }

    setRefreshEditorKey2(pre => pre + 1);
    setConfirmLoading(false);
    setTemplateVisible(false);
  };

  const systemMessage = (
    <FormPromptEditor
      fieldClassName="!pt-0"
      refreshEditorKey={refreshEditorKey + refreshEditorKey2}
      field={
        'current_version.evaluator_content.prompt_evaluator.message_list[0]'
      }
      disabled={disabled}
      noLabel
      rules={[{ required: true, message: I18n.t('system_prompt_not_empty') }]}
      minHeight={300}
      maxHeight={500}
      dragBtnHidden
      modalVariableEnable={multiModalVariableEnable}
      messageTypeDisabled={true}
      messageTypeList={messageTypeList}
      message={systemMsg}
      onMessageChange={m => {
        const messageList = [...(promptEvaluator?.message_list || [])];
        messageList[0] = m;
        promptEvaluatorFieldApi.setValue({
          ...promptEvaluator,
          message_list: messageList,
        });
      }}
    />
  );

  const userMessage = promptEvaluator?.message_list?.[1] ? (
    <FormPromptEditor
      fieldClassName="!pt-0"
      refreshEditorKey={refreshEditorKey + refreshEditorKey2}
      field={
        'current_version.evaluator_content.prompt_evaluator.message_list[1]'
      }
      noLabel
      disabled={disabled}
      rules={[{ required: true, message: I18n.t('user_prompt_required') }]}
      maxHeight={500}
      dragBtnHidden
      modalVariableEnable={multiModalVariableEnable}
      messageTypeDisabled={true}
      messageTypeList={messageTypeList}
      message={userMsg}
      onMessageChange={m => {
        const messageList = promptEvaluator?.message_list || [];
        messageList[1] = m;
        promptEvaluatorFieldApi.setValue({
          ...promptEvaluator,
          message_list: messageList,
        });
      }}
      rightActionBtns={
        <Popconfirm
          title={I18n.t('delete_user_prompt')}
          content={I18n.t('confirm_delete_user_prompt')}
          okText={I18n.t('confirm')}
          cancelText={I18n.t('cancel')}
          okButtonProps={{ color: 'red' }}
          onConfirm={() => {
            const messageList = promptEvaluator?.message_list || [];
            promptEvaluatorFieldApi.setValue({
              ...promptEvaluator,
              message_list: messageList.slice(0, 1),
            });
          }}
        >
          <Button
            color="secondary"
            size="mini"
            disabled={disabled}
            icon={<IconCozTrashCan />}
          />
        </Popconfirm>
      }
    />
  ) : (
    <Button
      color="primary"
      className="!w-full mb-3"
      onClick={() => {
        const messageList = promptEvaluator?.message_list || [];
        messageList[1] = {
          role: Role.User,
          content: {
            content_type: ContentType.Text,
            text: '',
          },
        };
        promptEvaluatorFieldApi.setValue({
          ...promptEvaluator,
          message_list: messageList,
        });
      }}
      disabled={disabled}
      icon={<IconCozPlus />}
    >
      {I18n.t('add_user_prompt')}
    </Button>
  );

  return (
    <>
      <div className={cls('py-[10px]', styles['prompt-field-wrapper'])}>
        <div className="flex flex-row items-center justify-between mb-1">
          <Form.Label required text={'Prompt'} className="!mb-1" />
          <div className="flex flex-row items-center">
            <Button
              size="mini"
              color="secondary"
              className="!coz-fg-hglt !px-[3px] !h-5"
              // disabled={disabled}
              // todo icon
              icon={<IconCozTemplate />}
              onClick={() => setTemplateVisible(true)}
            >
              {I18n.t('select_template')}
              {promptEvaluator?.prompt_template_name
                ? `(${promptEvaluator.prompt_template_name})`
                : ''}
            </Button>

            <Divider layout="vertical" className="h-3 mx-2" />

            {disabled ? (
              <Button
                size="mini"
                color="secondary"
                className="!px-[3px] !h-5"
                icon={<IconCozTrashCan />}
                disabled={disabled}
              >
                {I18n.t('clear')}
              </Button>
            ) : (
              <Popconfirm
                title={I18n.t('confirm_clear_prompt')}
                cancelText={I18n.t('cancel')}
                okText={I18n.t('clear')}
                okButtonProps={{ color: 'red' }}
                onConfirm={() => {
                  promptEvaluatorFieldApi.setValue({
                    model_config:
                      promptEvaluatorFieldApi.getValue()?.model_config,
                    message_list: [
                      {
                        role: Role.System,
                        content: {
                          content_type: 'Text',
                          text: '',
                        },
                      },
                    ],
                  });
                  setRefreshEditorKey2(pre => pre + 1);
                }}
              >
                <Button
                  size="mini"
                  color="secondary"
                  className="!px-[3px] !h-5"
                  icon={<IconCozTrashCan />}
                >
                  {I18n.t('clear')}
                </Button>
              </Popconfirm>
            )}
          </div>
        </div>
        {systemMessage}
        {userMessage}
        {variables?.length ? (
          <PromptVariablesList variables={variables} />
        ) : null}
      </div>

      <TemplateModal
        visible={templateVisible}
        disabled={disabled}
        confirmLoading={confirmLoading}
        onCancel={() => setTemplateVisible(false)}
        onSelect={handleTemplateSelect}
      />
    </>
  );
}

const FormPromptEditor = withField(
  (
    props: EvaluatorPromptEditorProps & {
      refreshEditorKey?: number;
    },
  ) => <EvaluatorPromptEditor {...props} key={props.refreshEditorKey} />,
);

/* 提交表单时再获取 inputSchema */
export function generateInputSchemas(messageList?: Message[]) {
  const variables = parseMessagesVariables(messageList ?? []);
  const inputSchema = variables.map(variable => {
    const schema: common.ArgsSchema = {
      key: variable.key,
    };
    if (variable.type === VariableType.String) {
      schema.support_content_types = [ContentType.Text];
      schema.json_schema = '{"type": "string"}';
    } else if (variable.type === VariableType.MultiPart) {
      schema.support_content_types = [ContentType.MultiPart];
    }
    return schema;
  });

  return inputSchema;
}
