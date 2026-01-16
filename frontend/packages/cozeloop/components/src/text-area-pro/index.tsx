// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { useState } from 'react';

import { IconCozContent } from '@coze-arch/coze-design/icons';
import {
  Button,
  Modal,
  TextArea,
  type TextAreaProps,
} from '@coze-arch/coze-design';

import s from './index.module.less';

export interface Props extends TextAreaProps {
  bottomExtra?: React.ReactNode;
}

export function TextAreaPro({ bottomExtra, ...props }: Props) {
  const [full, setFull] = useState(false);

  return (
    <div className="relative">
      {full ? (
        <Modal
          motion={false}
          title=" "
          fullScreen
          visible={true}
          onCancel={() => setFull(false)}
          footer={null}
        >
          <TextArea {...props} rows={undefined} className={s.full} />
        </Modal>
      ) : null}

      <TextArea {...props} />
      <div className="absolute bottom-1 left-1 z-10 h-8 flex flex-row items-center">
        <Button
          className="z-10 h-8 w-8"
          icon={<IconCozContent />}
          size="small"
          color="secondary"
          type="tertiary"
          onClick={() => setFull(true)}
        ></Button>
        {bottomExtra}
      </div>
    </div>
  );
}
