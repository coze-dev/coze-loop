// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */

import { useRef, useImperativeHandle, forwardRef } from 'react';

import { cloneDeep } from 'lodash-es';
import cls from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { tag } from '@cozeloop/api-schema/data';
import { IconCozTrashCan, IconCozPlus } from '@coze-arch/coze-design/icons';
import {
  Form,
  FormInput,
  FormTextArea,
  Divider,
  FormSelect,
  ArrayField,
  Button,
  Switch,
  withField,
  Tooltip,
  type FormState,
  type SwitchProps,
  Toast,
} from '@coze-arch/coze-design';

import {
  composeValidate,
  tagNameValidate,
  tagValidateNameUniqByOptions,
  useTagNameValidateUniqBySpace,
  type ValidateFn,
} from '@/utils/validate';
import { TAG_TYPE_OPTIONS, MAX_TAG_LENGTH } from '@/const';

const { TagContentType } = tag;

interface FormSwitchProps {
  value?: boolean;
  onChange?: (value: boolean) => void;
  size?: SwitchProps['size'];
  disabled?: boolean;
}
const FormSwitch = withField(
  ({ value, onChange, size, disabled }: FormSwitchProps) => (
    <Switch
      checked={value}
      onChange={onChange}
      size={size}
      disabled={disabled}
    />
  ),
);

interface TagValue extends tag.TagValue {
  tag_status: boolean;
}

export interface FormValues extends tag.TagInfo {
  tag_values: TagValue[];
}

interface TagsForm {
  className?: string;
  entry: 'crete-tag' | 'edit-tag';
  onChange?: (value?: FormState<FormValues>) => void;
  onSubmit?: (values: FormValues) => void;
  defaultValues?: tag.TagInfo;
  maxTags?: number;
  onValueChange?: (values: FormValues) => void;
}

export interface TagFormRef {
  submit: () => void;
}

export const TagsForm = forwardRef((props: TagsForm, ref) => {
  const {
    className,
    onChange,
    onSubmit,
    defaultValues,
    entry,
    maxTags = MAX_TAG_LENGTH,
    onValueChange,
  } = props;
  const validateUniqBySpace = useTagNameValidateUniqBySpace(
    defaultValues?.tag_key_id,
  );

  const isEditMode = entry === 'edit-tag';

  const originValues = cloneDeep(defaultValues);

  const formRef = useRef<Form<FormValues>>(null);
  useImperativeHandle(ref, () => ({
    submit: () => formRef.current?.formApi.submitForm(),
  }));
  return (
    <div className={cls('w-full', className)}>
      <Form<FormValues>
        ref={formRef}
        onChange={formState => {
          onChange?.(formState);
        }}
        onSubmit={values => {
          if (
            values.content_type === TagContentType.Categorical &&
            (!values.tag_values || values.tag_values.length === 0)
          ) {
            Toast.error(I18n.t('add_at_least_one_option'));
            return;
          }
          onSubmit?.(values);
        }}
        initValues={defaultValues as FormValues}
        onValueChange={(value, changedValue) => {
          onValueChange?.(value as FormValues);
        }}
      >
        {({ formState, formApi }) => (
          <>
            <div className="text-[16px] font-semibold leading-[22px] text-[var(--coz-fg-primary)] mb-3">
              {I18n.t('basic_info')}
            </div>
            <FormInput
              field="tag_key_name"
              label={I18n.t('tag_name')}
              placeholder={I18n.t('enter_tag_name')}
              rules={[{ required: true, message: I18n.t('enter_tag_name') }]}
              maxLength={50}
              validate={composeValidate([
                tagNameValidate,
                validateUniqBySpace as ValidateFn,
              ])}
              trigger="blur"
            />

            <FormTextArea
              field="description"
              label={I18n.t('description')}
              placeholder={I18n.t('enter_description')}
              maxCount={200}
              maxLength={200}
              validate={value => {
                if (value && value.length > 200) {
                  return I18n.t('description_length_limit');
                }
                return '';
              }}
            />

            <Divider className="my-3" />
            <div className="text-[16px] font-semibold leading-[22px] text-[var(--coz-fg-primary)] mb-3 pt-3">
              {I18n.t('tag_configuration')}
            </div>
            <FormSelect
              field="content_type"
              label={I18n.t('tag_type')}
              className="w-full"
              placeholder={I18n.t('enter_tag_type')}
              rules={[{ required: true, message: I18n.t('enter_tag_type') }]}
              optionList={TAG_TYPE_OPTIONS}
              disabled={entry === 'edit-tag'}
              onChange={value => {
                if (value === tag.TagContentType.Boolean) {
                  formApi.setValue('tag_values', [
                    {
                      tag_value_name: I18n.t('yes'),
                    },
                    {
                      tag_value_name: I18n.t('no'),
                    },
                  ]);
                } else {
                  formApi.setValue('tag_values', undefined);
                }
              }}
            />

            <>
              {formState.values.content_type === TagContentType.Categorical ? (
                <ArrayField field="tag_values">
                  {({ add, arrayFields }) => (
                    <div className="w-full flex flex-col gap-y-3 mt-2">
                      {arrayFields.map(({ field, key, remove }, index) => {
                        const currentValueItem =
                          formState.values.tag_values?.[index];

                        const originValueItem = originValues?.tag_values?.find(
                          tagItem =>
                            tagItem.tag_value_id ===
                            currentValueItem?.tag_value_id,
                        );

                        const isChanged =
                          originValueItem !== undefined &&
                          originValueItem?.tag_value_name !==
                            currentValueItem?.tag_value_name;
                        const tagNames =
                          formState.values?.tag_values?.map(
                            item => item.tag_value_name,
                          ) ?? [];

                        return (
                          <div
                            key={key}
                            className="px-3 py-4 rounded-[12px] coz-bg-primary"
                          >
                            <div className="flex items-center overflow-hidden gap-x-2 w-full max-w-[800px]">
                              <div className="flex-1 max-w-full ">
                                <FormInput
                                  className="w-full"
                                  noLabel
                                  field={`${field}.tag_value_name`}
                                  placeholder={I18n.t('please_enter')}
                                  fieldClassName="!py-0"
                                  maxLength={50}
                                  onChange={() => {
                                    formApi
                                      .validate(['tag_values'])
                                      .catch(console.log);
                                  }}
                                  disabled={
                                    !currentValueItem?.tag_status &&
                                    isEditMode &&
                                    currentValueItem?.tag_key_id !== undefined
                                  }
                                  validate={composeValidate([
                                    tagNameValidate,
                                    tagValidateNameUniqByOptions(
                                      tagNames,
                                      index,
                                    ),
                                  ])}
                                />
                              </div>
                              {isEditMode &&
                              currentValueItem?.tag_key_id !== undefined ? (
                                <Tooltip
                                  theme="dark"
                                  content={
                                    currentValueItem?.tag_status
                                      ? I18n.t('disable_tag')
                                      : I18n.t('enable_tag')
                                  }
                                >
                                  <FormSwitch
                                    noLabel
                                    size="mini"
                                    field={`${field}.tag_status`}
                                  />
                                </Tooltip>
                              ) : (
                                <Tooltip
                                  content={I18n.t('delete')}
                                  theme="dark"
                                >
                                  <Button
                                    onClick={() => remove()}
                                    color="secondary"
                                    size="small"
                                    icon={
                                      <IconCozTrashCan className="w-[14px] h-[14px]" />
                                    }
                                  />
                                </Tooltip>
                              )}
                            </div>
                            {isChanged ? (
                              <div className="flex items-center text-[12px] leading-4 font-normal text-[var(--coz-fg-secondary)] mt-2">
                                <span>{I18n.t('before_modification')}</span>
                                <span>{originValueItem?.tag_value_name}</span>
                              </div>
                            ) : null}
                          </div>
                        );
                      })}
                      <Button
                        onClick={add}
                        disabled={arrayFields.length >= Number(maxTags)}
                        className="w-full sticky bottom-0 !coz-bg-secondary"
                        color="primary"
                        icon={<IconCozPlus />}
                      >
                        <span>{I18n.t('add_tag_option')}</span>
                        <div className="coz-fg-dim ml-1">
                          {arrayFields.length}/{maxTags}
                        </div>
                      </Button>
                    </div>
                  )}
                </ArrayField>
              ) : null}
            </>
            <>
              {formState.values.content_type === TagContentType.Boolean && (
                <div className="px-3 py-4 rounded-[12px] bg-[var(--coz-bg-primary)] flex flex-col gap-y-2">
                  <div className="flex items-center gap-x-3 w-full">
                    <span className="text-[var(--coz-fg-primary)] text-[14px] font-normal leading-5">
                      {I18n.t('tag_option_one')}
                    </span>
                    <FormInput
                      field="tag_values.0.tag_value_name"
                      placeholder={I18n.t('please_enter')}
                      fieldClassName="!py-0 flex-1"
                      noLabel
                      maxLength={50}
                      validate={composeValidate([
                        tagNameValidate,
                        tagValidateNameUniqByOptions(
                          formState.values.tag_values?.map(
                            item => item.tag_value_name,
                          ) ?? [],
                          0,
                        ),
                      ])}
                      onChange={() => {
                        formApi.validate(['tag_values']).catch(console.log);
                      }}
                    />
                  </div>
                  <div className="flex items-center gap-x-3 w-full">
                    <span className="text-[var(--coz-fg-primary)] text-[14px] font-normal leading-5">
                      {I18n.t('tag_option_two')}
                    </span>
                    <FormInput
                      field="tag_values.1.tag_value_name"
                      placeholder={I18n.t('please_enter')}
                      fieldClassName="!py-0 flex-1"
                      noLabel
                      maxLength={50}
                      onChange={() => {
                        formApi.validate(['tag_values']).catch(console.log);
                      }}
                      validate={composeValidate([
                        tagNameValidate,
                        tagValidateNameUniqByOptions(
                          formState.values.tag_values?.map(
                            item => item.tag_value_name,
                          ) ?? [],
                          1,
                        ),
                      ])}
                    />
                  </div>
                </div>
              )}
            </>
          </>
        )}
      </Form>
    </div>
  );
});
