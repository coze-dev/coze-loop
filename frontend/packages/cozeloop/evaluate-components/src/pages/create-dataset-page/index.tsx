// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { useBreadcrumb } from '@cozeloop/hooks';
import { RouteBackAction } from '@cozeloop/components';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { Layout, Typography } from '@coze-arch/coze-design';

import { DatasetCreateForm } from '../../components/dataset-create-form';

export const CreateDatasetPage = () => {
  const navigate = useNavigateModule();
  useBreadcrumb({
    text: I18n.t('create_evaluation_set'),
  });

  return (
    <Layout.Content className="h-full w-full overflow-hidden flex flex-col">
      <DatasetCreateForm
        header={
          <div className="flex items-center gap-2 ">
            <RouteBackAction onBack={() => navigate('evaluation/datasets')} />
            <Typography.Title
              heading={6}
              className="!coz-fg-plus !font-medium !text-[18px] !leading-[20px]"
            >
              {I18n.t('create_evaluation_set')}
            </Typography.Title>
          </div>
        }
      />
    </Layout.Content>
  );
};
