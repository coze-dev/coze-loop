import { type FieldSchema } from '@cozeloop/api-schema/evaluation';

import {
  EqualItem,
  getSchemaTypeText,
  getTypeText,
  ReadonlyItem,
} from '../column-item-map';
import { schemaSourceTypeMap, type OptionSchema } from './types';

export function ReadonlyMappingItem({
  keyTitle,
  keySchema,
  optionSchema,
}: {
  keyTitle?: string;
  keySchema?: FieldSchema;
  optionSchema?: OptionSchema;
}) {
  return (
    <div className="flex flex-row items-center gap-2">
      <ReadonlyItem
        className="flex-1 basis-80 overflow-hidden"
        title={keyTitle}
        typeText={getSchemaTypeText(keySchema)}
        value={keySchema?.name}
      />
      <EqualItem />
      <ReadonlyItem
        className="flex-1 basis-80 overflow-hidden"
        title={
          optionSchema?.schemaSourceType &&
          schemaSourceTypeMap[optionSchema.schemaSourceType]
        }
        typeText={getTypeText(optionSchema)}
        value={optionSchema?.name}
      />
    </div>
  );
}
