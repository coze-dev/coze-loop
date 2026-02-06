// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useBlocker } from 'react-router-dom';
import { useState, useEffect } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { Modal } from '@coze-arch/coze-design';

export const useLeaveGuard = () => {
  const [blockLeave, setBlockLeave] = useState(false);

  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      currentLocation.pathname !== nextLocation.pathname && blockLeave,
  );

  useEffect(() => {
    if (blocker.state === 'blocked') {
      Modal.warning({
        title: I18n.t('information_unsaved'),
        content: I18n.t('leave_page_tip'),
        cancelText: I18n.t('cancel'),
        onCancel: blocker.reset,
        okText: I18n.t('global_btn_confirm'),
        onOk: blocker.proceed,
      });
    }
  }, [blocker.state]);

  return {
    blockLeave,
    setBlockLeave,
  };
};
