// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { DurationDay } from '@cozeloop/api-schema/foundation';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import {
  Form,
  Input,
  Modal,
  Tooltip,
  type FormApi,
} from '@coze-arch/coze-design';

import { disabledDate, formatDate, getExpirationOptions } from './utils';

import s from './pat-modal.module.less';

export interface PatInfo {
  id?: string;
  name: string;
  duration?: DurationDay | 'custom';
  expireAt?: Date;
}

interface Props {
  visible?: boolean;
  title?: string;
  value?: PatInfo;
  onUpdate?: (v: PatInfo) => void;
  onCreate?: (v: PatInfo) => void;
  onCancel?: () => void;
}

export function PatModal({
  visible,
  title,
  value,
  onUpdate,
  onCreate,
  onCancel,
}: Props) {
  const [name, setName] = useState<string>('Secret Token');
  const formApi = useRef<FormApi<PatInfo>>();
  const [duration, setDuration] = useState<PatInfo['duration']>();
  const dataOptionsList = getExpirationOptions();
  const isCreate = !value;
  const modalTitle = title || isCreate ? I18n.t('add_pat') : I18n.t('edit_pat');

  const onOk = async () => {
    const values = await formApi.current?.getValues();
    if (!values) {
      return;
    }

    await (isCreate ? onCreate?.(values) : onUpdate?.(values));
  };

  useEffect(() => {
    value && formApi.current?.setValue('name', value.name);
  }, [value]);

  return (
    <Modal
      visible={visible}
      title={modalTitle}
      okText={I18n.t('confirm')}
      cancelText={I18n.t('cancel')}
      onOk={onOk}
      okButtonProps={{ disabled: !name }}
      onCancel={onCancel}
    >
      <Form<PatInfo>
        getFormApi={api => (formApi.current = api)}
        showValidateIcon={false}
        initValues={{ name: 'Secret Token', duration: DurationDay.Day1 }}
        onValueChange={v => setName(v.name)}
      >
        <Form.Input
          trigger={['blur', 'change']}
          field="name"
          label={{ text: I18n.t('name'), required: true }}
          placeholder={'Input token name'}
          maxLength={20}
        />
        <Form.Slot
          label={{
            text: I18n.t('expiration_time'),
            required: true,
            extra: (
              <Tooltip
                theme="dark"
                trigger="hover"
                content={I18n.t('expired_time_tip')}
              >
                <div
                  className={
                    'flex items-center justify-center hover:coz-mg-secondary-hovered w-[16px] h-[16px] rounded-[4px] mr-[4px] ml-[2px] text-[12px]'
                  }
                >
                  <IconCozInfoCircle className="coz-fg-secondary" />
                </div>
              </Tooltip>
            ),
          }}
        >
          {isCreate ? (
            <div className={s['expiration-select']}>
              <Form.Select
                noLabel={true}
                field="duration"
                style={{ width: '100%' }}
                disabled={!isCreate}
                optionList={dataOptionsList}
                onChange={v => setDuration(v as PatInfo['duration'])}
                placeholder={I18n.t('please_select', {
                  field: I18n.t('expiration_time'),
                })}
              />
              {duration === 'custom' ? (
                <Form.DatePicker
                  noLabel={true}
                  field="expireAt"
                  style={{ width: '100%' }}
                  disabled={!isCreate}
                  disabledDate={disabledDate}
                  position="bottomRight"
                />
              ) : null}
            </div>
          ) : (
            <Input disabled value={formatDate(value.expireAt)} />
          )}
        </Form.Slot>
      </Form>
    </Modal>
  );
}
