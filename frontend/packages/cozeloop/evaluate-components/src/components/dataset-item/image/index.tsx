import { useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozEye, IconCozDownload } from '@coze-arch/coze-design/icons';
import { Image, ImagePreview } from '@coze-arch/coze-design';

import { downloadWithUrl } from '@/utils/download-template';

import { type DatasetItemProps } from '../type';

export const ImageDatasetItem: React.FC<
  DatasetItemProps & { size?: number; disableDownload?: boolean }
> = ({
  fieldContent,
  expand,
  onChange,
  className,
  size = 36,
  disableDownload = false,
}) => {
  const { image } = fieldContent || {};
  const [visible, setVisible] = useState(false);
  return (
    <div
      className={`inline-block relative group ${className}`}
      onClick={e => {
        e.stopPropagation();
      }}
    >
      <ImagePreview
        src={image?.url}
        visible={visible}
        onVisibleChange={setVisible}
      />
      <Image
        src={image?.url}
        alt={image?.name}
        width={'100%'}
        height={'100%'}
        imgStyle={{
          objectFit: 'cover',
          objectPosition: 'center',
        }}
      />
      <div
        className={
          'invisible absolute inset-0 flex gap-3 items-center rounded-[6px] justify-center bg-[rgba(0,0,0,0.4)]  group-hover:visible'
        }
      >
        {image?.url ? (
          <IconCozEye
            className="text-white w-[16px] h-[16px] cursor-pointer"
            onClick={() => setVisible(true)}
          />
        ) : null}
        {disableDownload ? null : (
          <IconCozDownload
            className="text-white w-[16px] h-[16px] cursor-pointer"
            onClick={() => {
              if (image?.url) {
                downloadWithUrl(image?.url, image?.name || I18n.t('test'));
              }
            }}
          />
        )}
      </div>
    </div>
  );
};
