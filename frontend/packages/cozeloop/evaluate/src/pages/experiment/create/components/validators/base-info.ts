/* eslint-disable @typescript-eslint/no-explicit-any */
import { sourceNameRuleValidator } from '@cozeloop/evaluate-components';

import { checkExperimentName } from '@/request/experiment';

export const baseInfoValidators: Record<string, any[]> = {
  name: [
    { required: true, message: '请输入名称' },
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
              err = new Error('名称已存在');
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
