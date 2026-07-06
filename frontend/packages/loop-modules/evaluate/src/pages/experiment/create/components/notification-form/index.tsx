// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  FormInput,
  Switch,
  Tooltip,
  useFormState,
  withField,
  type SwitchProps,
} from '@coze-arch/coze-design';

import { type CreateExperimentValues } from '@/types/experiment/experiment-create';

const WEBHOOK_URL_MAX = 10;
const URL_PLACEHOLDER =
  '请输入 http(s):// 开头的 Webhook URL,单实验最多 10 条(以英文逗号分隔)';
const WEBHOOK_HINT = '启用后，实验状态变化将向下列 URL 发送 POST 通知';
const FEISHU_HINT =
  '飞书通知将发送给实验创建人；通过 CLI/API 创建的实验若无创建人信息将不发送';

interface EnableSwitchProps {
  value?: boolean;
  onChange?: (v: boolean) => void;
  size?: SwitchProps['size'];
}

const FormSwitch = withField(
  ({ value, onChange, size }: EnableSwitchProps) => (
    <Switch checked={value} onChange={onChange} size={size} />
  ),
);

const validateWebhookUrls = (raw?: string): boolean | string => {
  if (!raw) {
    return 'Webhook URL 不能为空';
  }
  const list = raw
    .split(',')
    .map(s => s.trim())
    .filter(Boolean);
  if (list.length > WEBHOOK_URL_MAX) {
    return '已达单实验 Webhook URL 上限(10 条)';
  }
  for (const url of list) {
    if (!/^https?:\/\//.test(url)) {
      return 'URL 需以 http(s):// 开头';
    }
  }
  return true;
};

export const NotificationForm = () => {
  const formState = useFormState();
  const values = formState.values as CreateExperimentValues;
  const webhookEnabled = Boolean(
    values?.notification_conf?.webhook?.enable,
  );

  return (
    <div className="flex flex-col gap-3 mt-3">
      <div className="text-[16px] leading-[22px] font-medium coz-fg-primary">
        通知配置
      </div>
      <Tooltip content={WEBHOOK_HINT}>
        <FormSwitch
          field="notification_conf.webhook.enable"
          label="Webhook 通知"
        />
      </Tooltip>
      {webhookEnabled ? (
        <FormInput
          field="notification_conf.webhook.urls"
          label="Webhook URL"
          placeholder={URL_PLACEHOLDER}
          rules={[{ validator: (_, v) => validateWebhookUrls(v) }]}
        />
      ) : null}
      <Tooltip content={FEISHU_HINT}>
        <FormSwitch
          field="notification_conf.feishu_notification.enable"
          label="飞书通知"
        />
      </Tooltip>
    </div>
  );
};
