import { type DatasetItemProps } from '../../type';
import { TextEllipsis } from '../../../text-ellipsis';

export const BoolDatasetItemReadOnly = ({
  fieldContent,
  displayFormat,
}: DatasetItemProps) => (
  <div
    style={
      displayFormat
        ? {
            border: '1px solid var(--coz-stroke-primary)',
            borderRadius: '6px',
            backgroundColor: 'var(--coz-bg-plus)',
            padding: 12,
            minHeight: 48,
          }
        : {}
    }
  >
    <TextEllipsis emptyText="" theme="light">
      {fieldContent?.text}
    </TextEllipsis>
  </div>
);
