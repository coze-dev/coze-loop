import { tag } from '@cozeloop/api-schema/data';

const { TagStatus } = tag;

export const formatTagDetailToFormValues = (tagDetail: tag.TagInfo) => ({
  ...tagDetail,
  tag_values:
    tagDetail.tag_values?.map(value => ({
      ...value,
      tag_status: value.status === TagStatus.Active,
    })) || [],
});