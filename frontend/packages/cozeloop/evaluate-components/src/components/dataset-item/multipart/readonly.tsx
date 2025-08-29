import cn from 'classnames';
import { FieldDisplayFormat } from '@cozeloop/api-schema/data';
import { Popover, Tag } from '@coze-arch/coze-design';

import { ContentType, type DatasetItemProps } from '../type';
import styles from '../text/string/index.module.less';
import { StringDatasetItem } from '../text/string';
import { ImageDatasetItem } from '../image';
import { AudioDatasetItem } from '../audio';
const MultipartItemComponentMap = {
  [ContentType.Image]: ImageDatasetItem,
  [ContentType.Audio]: AudioDatasetItem,
  [ContentType.Text]: StringDatasetItem,
};

export const MultipartDatasetItemReadOnly: React.FC<
  DatasetItemProps
> = props => {
  const {
    fieldContent,
    expand,
    displayFormat,
    className: classNameProps,
  } = props;
  const { multi_part } = fieldContent || {};

  return !expand ? (
    <SlimMultipartDatasetItem {...props} />
  ) : (
    <div
      className={cn(
        'flex flex-wrap gap-[6px] max-h-[292px] overflow-y-auto',
        displayFormat && styles.border,
        classNameProps,
      )}
    >
      {multi_part?.map((item, index) => {
        if (!item.content_type) {
          return;
        }
        const isCode = fieldContent?.format === FieldDisplayFormat.Code;
        const className =
          item.content_type === ContentType.Text
            ? `w-full max-h-[auto] !border-0 ${!isCode ? '!p-0' : ''} !min-h-[22px]`
            : item.content_type === ContentType.Image
              ? 'w-[80px] h-[80px] overflow-hidden'
              : '';
        const Component =
          MultipartItemComponentMap[item.content_type] || StringDatasetItem;
        return (
          <Component
            key={index}
            fieldContent={{
              ...item,
              format: fieldContent?.format,
            }}
            expand={true}
            displayFormat={displayFormat}
            isEdit={false}
            className={className}
          />
        );
      })}
    </div>
  );
};
export const MAX_SHOW_ITEM = 4;

export const SlimMultipartDatasetItem: React.FC<DatasetItemProps> = props => {
  const { fieldContent } = props;
  const { multi_part } = fieldContent || {};
  if (multi_part?.length === 0) {
    return '';
  }
  if (multi_part?.[0]?.content_type === ContentType.Text) {
    return (
      <div className="flex items-center gap-[6px]">
        <div className="overflow-hidden">
          <StringDatasetItem
            fieldContent={multi_part?.[0]}
            expand={false}
            isEdit={false}
            displayFormat={false}
          />
        </div>
        {multi_part?.length > 1 && (
          <Popover
            content={
              <div className="p-3 w-[320px] max-w-[800px] max-h-[600px]">
                <MultipartDatasetItemReadOnly
                  {...props}
                  expand={true}
                  displayFormat={false}
                />
              </div>
            }
          >
            <Tag
              size="small"
              color="primary"
              className="min-w-[30px] !rounded-[99px] font-medium"
            >
              +{multi_part?.length - 1}
            </Tag>
          </Popover>
        )}
      </div>
    );
  } else {
    let leftCount = 0;
    return (
      <div className="flex gap-[6px]">
        {multi_part?.map((item, index) => {
          if (!item.content_type || leftCount !== 0) {
            return;
          }
          if (
            item.content_type === ContentType.Text ||
            index >= MAX_SHOW_ITEM
          ) {
            leftCount = multi_part.length - index;
            return;
          }
          const Component =
            MultipartItemComponentMap[item.content_type] || StringDatasetItem;
          const imageClassName =
            item.content_type === ContentType.Image ? 'w-[36px] h-[36px]' : '';
          return (
            <Component
              key={index}
              fieldContent={item}
              expand={true}
              displayFormat={true}
              className={imageClassName}
              disableDownload={true}
            />
          );
        })}

        {leftCount > 0 && (
          <Popover
            content={
              <div className="p-3 min-w-[320px] max-w-[800px] max-h-[600px]">
                <MultipartDatasetItemReadOnly {...props} expand={true} />
              </div>
            }
          >
            <Tag
              size="small"
              color="primary"
              className="min-w-[30px] !rounded-[99px] font-medium"
              onClick={e => {
                e.stopPropagation();
              }}
            >
              +{leftCount}
            </Tag>
          </Popover>
        )}
      </div>
    );
  }
};
