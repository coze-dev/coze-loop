import { useState } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { PageLoading } from '@cozeloop/components';
import { type ColumnAnnotation } from '@cozeloop/api-schema/evaluation';
import { type tag } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';
import { Toast } from '@coze-arch/coze-design';

import { TagSelect } from './tag-select';
import { AnnotateItemCard } from './annotate-item-card';

interface Props {
  spaceID: string;
  experimentID: string;
  data: ColumnAnnotation[];
  onAnnotateAdd?: () => void;
  onAnnotateDelete?: () => void;
}
export function AnnotateColSettings({
  spaceID,
  experimentID,
  data,
  onAnnotateAdd,
  onAnnotateDelete,
}: Props) {
  const [tags, setTags] = useState<tag.TagInfo[]>();

  const tagInit = useRequest(
    async () => {
      if (!data.length) {
        return [];
      }
      const res = await DataApi.BatchGetTags({
        workspace_id: spaceID,
        tag_key_ids: data.map(item => item.tag_key_id || '').filter(Boolean),
      });
      return res.tag_info_list || [];
    },
    {
      onSuccess: setTags,
    },
  );

  return (
    <div className="py-4">
      <div className="mb-8">
        <div className="text-[16px] font-semibold coz-fg-plus mb-3">
          {I18n.t('tag_list')}
        </div>
        <TagSelect
          spaceID={spaceID}
          experimentID={experimentID}
          className={'w-full'}
          dropdownClassName={'!p-3'}
          placeholder={I18n.t('please_enter_tag_name_search')}
          showTick={false}
          tags={tags}
          onAdd={tag => {
            setTags(prev => [...(prev || []), tag]);
            Toast.success(I18n.t('add_tag_column_to_data_details'));
            onAnnotateAdd?.();
          }}
        />
      </div>

      {tagInit.loading ? (
        <PageLoading className="mt-[120px]" />
      ) : (
        <div>
          {tags?.length ? (
            <div className="mb-4">
              {I18n.t('tag_added_placeholder1', { placeholder1: tags.length })}
            </div>
          ) : null}
          <div className="flex flex-col gap-3">
            {tags?.map(item => (
              <AnnotateItemCard
                key={item.tag_key_id}
                data={item}
                spaceID={spaceID}
                experimentID={experimentID}
                onDelete={tag => {
                  setTags(tags.filter(i => i.tag_key_id !== tag.tag_key_id));
                  onAnnotateDelete?.();
                }}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
