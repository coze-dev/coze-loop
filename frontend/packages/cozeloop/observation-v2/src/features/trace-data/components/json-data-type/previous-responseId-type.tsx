import { useState } from 'react';

import { type DataItemProps } from '@textea/json-viewer';
import { IconCozFixedSize } from '@coze-arch/coze-design/icons';

import { useLocale } from '@/i18n';

import { AnchorPanel } from '../anchor-panel';

export const PreviousResponseIdType = (props: DataItemProps<string>) => {
  const [visible, setVisible] = useState(false);
  const { t } = useLocale();

  return (
    <>
      <div className="inline-flex items-center justify-start flex-wrap relative">
        <span className="text-[#cb4b16] inline-flex items-center">
          {JSON.stringify(props.value)}
          <span
            className="inline-flex items-center cursor-pointer text-brand"
            onClick={() => setVisible(true)}
          >
            <IconCozFixedSize />
            <span>{t('view_context')}</span>
          </span>
        </span>
      </div>
      {visible ? (
        <AnchorPanel
          visible
          onClose={() => setVisible(false)}
          previousResponseId={props.value}
        />
      ) : null}
    </>
  );
};
