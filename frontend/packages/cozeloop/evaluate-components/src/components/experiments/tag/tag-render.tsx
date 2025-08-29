import {
  type AnnotateRecord,
  type ColumnAnnotation,
} from '@cozeloop/api-schema/evaluation';
import { tag } from '@cozeloop/api-schema/data';

import { TypographyText } from '@/components/text-ellipsis';

interface Props {
  annotation: ColumnAnnotation;
  /** 标注结果 */
  annotateRecord?: AnnotateRecord;

  className?: string;
}
export function TagRender({ annotation, annotateRecord, className }: Props) {
  const getContent = () => {
    switch (annotation.content_type) {
      case tag.TagContentType.FreeText:
        return annotateRecord?.plain_text;
      case tag.TagContentType.ContinuousNumber:
        return annotateRecord?.score ?? '-';
      case tag.TagContentType.Categorical:
        return (
          annotation.tag_values?.find(
            item => item.tag_value_id === annotateRecord?.tag_value_id,
          )?.tag_value_name ?? '-'
        );
      case tag.TagContentType.Boolean:
        return (
          annotation.tag_values?.find(
            item => item.tag_value_id === annotateRecord?.tag_value_id,
          )?.tag_value_name ?? '-'
        );
      default:
        break;
    }
  };

  return <TypographyText className={className}>{getContent()}</TypographyText>;
}
