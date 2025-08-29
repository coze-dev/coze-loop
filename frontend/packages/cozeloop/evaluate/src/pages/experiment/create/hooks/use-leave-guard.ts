import { useBlocker } from 'react-router-dom';
import { useState, useEffect } from 'react';

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
        title: '信息未保存',
        content: '离开当前页面，信息将不被保存。',
        cancelText: '取消',
        onCancel: blocker.reset,
        okText: '确认',
        onOk: blocker.proceed,
      });
    }
  }, [blocker.state]);

  return {
    blockLeave,
    setBlockLeave,
  };
};
