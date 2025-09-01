import {
  type EvaluationSetItem,
  type FieldData,
  type EvaluationSet,
} from '@cozeloop/api-schema/evaluation';

export const getDefaultEvaSetItem = (
  datasetDetail: EvaluationSet | undefined,
  spaceID: string,
): EvaluationSetItem => {
  const schema = datasetDetail?.evaluation_set_version?.evaluation_set_schema;
  const fieldDataList: FieldData[] =
    schema?.field_schemas?.map(field => ({
      key: field.key,
      name: field.name,
      content: {
        content_type: field.content_type,
      },
    })) || [];
  return {
    workspace_id: spaceID,
    evaluation_set_id: datasetDetail?.id,
    schema_id: schema?.id,
    turns: [
      {
        field_data_list: fieldDataList,
      },
    ],
  };
};
