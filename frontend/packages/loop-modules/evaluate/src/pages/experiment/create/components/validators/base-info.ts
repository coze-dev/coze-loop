// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { I18n } from '@cozeloop/i18n-adapter';
import { sourceNameRuleValidator } from '@cozeloop/evaluate-components';

import { checkExperimentName } from '@/request/experiment';

export const baseInfoValidators: Record<string, any[]> = {
  name: [
    { required: true, message: I18n.t('please_input_name') },
    { validator: sourceNameRuleValidator },
    {
      asyncValidator: async (_, value: string, spaceID: string) => {
        let err: Error | null = null;
        if (value) {
          try {
            const result = await checkExperimentName({
              workspace_id: spaceID,
              name: value,
            });
            if (!result.pass) {
              err = new Error(I18n.t('name_already_exists'));
            }
          } catch (e) {
            console.error('接口遇到问题', e);
          }
          if (err !== null) {
            throw err;
          }
        }
      },
    },
  ],

  desc: [],
};
