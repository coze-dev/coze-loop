// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Popconfirm, type PopconfirmProps } from '@coze-arch/coze-design';

interface Props extends PopconfirmProps {
  children?: React.ReactNode;
  needConfirm?: boolean;
}

export const PopconfirmSave: React.FC<Props> = props => {
  const { children, needConfirm, ...reset } = props;
  return props.needConfirm ? (
    <Popconfirm
      title={I18n.t('information_unsaved')}
      content={I18n.t('saved_lost_data_tips')}
      okText={I18n.t('evaluation_set_save_and_continue')}
      cancelText={I18n.t('do_not_save')}
      {...reset}
    >
      {props.children}
    </Popconfirm>
  ) : (
    props.children
  );
};
