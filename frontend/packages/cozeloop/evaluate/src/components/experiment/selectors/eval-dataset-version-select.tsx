import { useDebounceFn, useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Select, type SelectProps } from '@coze-arch/coze-design';

export default function EvalDatasetVersionSelect(
  props: { datasetId: string } & SelectProps,
) {
  const { spaceID } = useSpace();
  const service = useRequest(async () => {
    const res = await StoneEvaluationApi.ListEvaluationSetVersions({
      workspace_id: spaceID,
      page_size: 100,
      evaluation_set_id: props.datasetId,
    });
    return res.versions?.map(item => ({
      value: item.id,
      label: item.version,
      ...item,
    }));
  });

  const handleSearch = useDebounceFn(service.run, {
    wait: 500,
  });

  return (
    <Select
      placeholder="请选择评测集版本"
      filter={true}
      searchPosition="dropdown"
      searchPlaceholder="请输入"
      remote
      {...props}
      loading={service.loading}
      onSearch={handleSearch.run}
      optionList={service.data}
      onDropdownVisibleChange={visible => {
        if (visible) {
          service.refresh();
        }
      }}
    />
  );
}
