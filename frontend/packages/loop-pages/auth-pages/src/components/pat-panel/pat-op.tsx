// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cls from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { type PersonalAccessToken } from '@cozeloop/api-schema/foundation';
import { IconCozMinusCircle, IconCozEdit } from '@coze-arch/coze-design/icons';
import { IconButton, Popconfirm } from '@coze-arch/coze-design';

import s from './pat-op.module.less';

interface Props {
  className?: string;
  pat: PersonalAccessToken;
  onEdit?: (v: PersonalAccessToken) => void;
  onDelete?: (id: string) => void;
}

export function PatOperation({ pat, className, onEdit, onDelete }: Props) {
  return (
    <div className={cls(s.container, className)}>
      <IconButton
        icon={<IconCozEdit />}
        size="small"
        color="secondary"
        onClick={() => onEdit?.(pat)}
      />
      <Popconfirm
        trigger="click"
        title={I18n.t('delete_token')}
        content={I18n.t('remove_will_affect_all_in_use')}
        okText={I18n.t('confirm')}
        cancelText={I18n.t('cancel')}
        okButtonProps={{ color: 'red' }}
        style={{ width: 320 }}
        onConfirm={() => onDelete?.(pat.id)}
      >
        <IconButton
          icon={<IconCozMinusCircle />}
          size="small"
          color="secondary"
        />
      </Popconfirm>
    </div>
  );
}
