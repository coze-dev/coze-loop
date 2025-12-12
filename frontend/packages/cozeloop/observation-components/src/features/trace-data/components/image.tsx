// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { IconCozCrossCircleFill } from '@coze-arch/coze-design/icons';
import { Image, Tooltip, Tag } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';
import { useFetchResource } from '@/features/trace-data/hooks/use-fetch-resource';

export const TraceImage = ({ url }: { url: string }) => {
  const { error } = useFetchResource(url);
  const { t } = useLocale();

  if (error) {
    return (
      <Tooltip content={t('analytics_image_error')} theme="dark">
        <div className="flex items-center">
          <Tag type="solid" color="red">
            <span className="flex items-center gap-x-1">
              <IconCozCrossCircleFill />
              <span className="font-medium">{t('image_load_failed')}</span>
            </span>
          </Tag>
        </div>
      </Tooltip>
    );
  }
  return (
    <Image
      src={url}
      imgCls="max-h-[200px] w-auto"
      preview={{ closable: true }}
    />
  );
};
