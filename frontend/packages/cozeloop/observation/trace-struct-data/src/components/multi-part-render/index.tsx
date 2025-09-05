// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cn from 'classnames';
import { Typography, Image } from '@coze-arch/coze-design';

import { type MultiPartSchema } from '../../span-definition/prompt/schema';
import { MultiPartType, type DatasetItemProps } from './type';

const ImageDatasetItem: React.FC<DatasetItemProps> = ({
  fieldContent,
  className,
}) => {
  const { image_url } = fieldContent || {};

  return (
    <Image
      className={cn('inline-block', className)}
      src={image_url?.url}
      alt={image_url?.name}
      width={36}
      height={36}
    />
  );
};

function StringDatasetItem({ fieldContent, className }: DatasetItemProps) {
  const { text } = fieldContent || {};

  return (
    <Typography.Text
      style={{ color: 'inherit', fontSize: 'inherit' }}
      className={cn('max-h-[292px] overflow-y-auto break-all', className)}
    >
      {text}
    </Typography.Text>
  );
}

const MultipartItemComponentMap = {
  [MultiPartType.Text]: StringDatasetItem,
  [MultiPartType.ImageUrl]: ImageDatasetItem,
};

export function MultipartRender(props: {
  parts?: MultiPartSchema[];
  className?: string;
}) {
  const { parts } = props;

  return (
    <div
      className={cn(
        'flex flex-wrap gap-1 max-h-[292px] overflow-y-auto',
        props.className,
      )}
    >
      {parts?.map((item, index) => {
        if (!item.type) {
          return;
        }
        const className =
          item.type === MultiPartType.Text
            ? 'w-full max-h-[auto] !border-0 !p-0'
            : '';
        const Component =
          MultipartItemComponentMap[item.type] || StringDatasetItem;
        return (
          <Component
            key={index}
            fieldContent={item}
            expand={true}
            className={className}
          />
        );
      })}
    </div>
  );
}
