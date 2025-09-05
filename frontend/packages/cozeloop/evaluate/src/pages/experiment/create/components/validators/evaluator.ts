import { I18n } from '@cozeloop/i18n-adapter';
export const evaluatorValidators = {
  evaluatorProList: [
    { required: true, message: I18n.t('please_add_evaluator') },
    { type: 'array', min: 1, message: I18n.t('add_at_least_one_evaluator') },
    { type: 'array', max: 5, message: I18n.t('max_add_x_evaluators') },
  ],
};
