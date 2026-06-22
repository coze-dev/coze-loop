// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useDebounceFn } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import { UserSelect } from '@cozeloop/biz-components-adapter';
import {
  EvaluationSetTagRelation,
  type ListEvaluationSetsRequest,
} from '@cozeloop/api-schema/evaluation';
import { IconCozMagnifier } from '@coze-arch/coze-design/icons';
import { Form, Search, withField } from '@coze-arch/coze-design';

import { DatasetTagsFilterSelect } from './dataset-tags-filter-select';

interface ListFilterFormValues extends Partial<ListEvaluationSetsRequest> {
  tag_ids?: string[];
}

interface ListFilterProps {
  filter: Partial<ListEvaluationSetsRequest>;
  setFilter: (filter: Partial<ListEvaluationSetsRequest>) => void;
}
const FormUserSelect = withField(UserSelect);
const FormNameSearch = withField(Search);
const FormTagsFilterSelect = withField(DatasetTagsFilterSelect);

export const ListFilter = ({ filter, setFilter }: ListFilterProps) => {
  const { data: guardData } = useGuard({
    point: GuardPoint['eval.datasets.search_by_creator'],
  });
  const { run } = useDebounceFn(
    (values: ListFilterFormValues) => {
      const { tag_ids: tagIDs, ...restValues } = values;
      setFilter({
        ...restValues,
        name: values?.name?.trim(),
        tag_filter: tagIDs?.length
          ? {
              tag_ids: tagIDs,
              relation: EvaluationSetTagRelation.Or,
            }
          : undefined,
      });
    },
    {
      wait: 500,
    },
  );
  return (
    <Form<ListFilterFormValues>
      layout="horizontal"
      onValueChange={run}
      initValues={{
        name: filter?.name,
        creators: filter?.creators,
        tag_ids: filter?.tag_filter?.tag_ids,
      }}
    >
      <div className="w-60 mr-2">
        <FormNameSearch
          noLabel
          field="name"
          fieldClassName="!mr-0 !pr-0"
          className="!w-full"
          placeholder={I18n.t('search_name')}
          prefix={<IconCozMagnifier />}
          convert={value => value?.slice(0, 100)}
          showClear
          autoComplete="off"
        />
      </div>

      <div className="w-60 mr-2">
        <FormTagsFilterSelect
          noLabel
          field="tag_ids"
          fieldClassName="!mr-0 !pr-0"
          className="!w-full"
        />
      </div>

      {!guardData.readonly && <FormUserSelect noLabel field="creators" />}
    </Form>
  );
};
