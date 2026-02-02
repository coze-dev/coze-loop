// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useDebounceFn } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import { UserSelect } from '@cozeloop/biz-components-adapter';
import { IconCozMagnifier } from '@coze-arch/coze-design/icons';
import { Form, Search, withField } from '@coze-arch/coze-design';

import { type FilterParams } from './types';

const FormUserSelect = withField(UserSelect);
const FormNameSearch = withField(Search);

export function EvaluatorListFilter({
  filterParams,
  onFilter,
}: {
  filterParams?: FilterParams;
  onFilter?: (val: FilterParams) => void;
}) {
  const { data: guardData } = useGuard({
    point: GuardPoint['eval.evaluators.search_by_creator'],
  });

  const { run } = useDebounceFn(
    (values: FilterParams) => {
      onFilter?.({ ...values });
    },
    {
      wait: 500,
    },
  );

  return (
    <Form<FilterParams>
      layout="horizontal"
      initValues={filterParams}
      onValueChange={run}
    >
      <div className="w-60 mr-2">
        <FormNameSearch
          noLabel
          prefix={<IconCozMagnifier />}
          field="search_name"
          placeholder={I18n.t('search_name')}
          fieldClassName="!mr-0 !pr-0"
          className="!w-full"
          autoComplete="off"
        />
      </div>
      {!guardData.readonly && (
        <FormUserSelect
          noLabel
          field="creator_ids"
          fieldClassName="!pr-[8px]"
        />
      )}

      {/* <FormSelect
        noLabel
        field="evaluator_type"
        className="w-[144px]"
        multiple
        placeholder={'所有类型'}
        optionList={evaluatorTypeOptions}
        // showClear
      /> */}
    </Form>
  );
}
