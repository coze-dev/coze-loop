import { i18nService } from '@/i18n';

const MAX_NAME_LENGTH = 20;
export const validateViewName = (name: string, viewNames: string[]) => {
  if (name.trim() === '') {
    return {
      isValid: false,
      message: i18nService.t('not_allowed_to_be_empty'),
    };
  }

  if (name.trim().length > MAX_NAME_LENGTH) {
    return {
      isValid: false,
      message: i18nService.t('name_length_limit', { num: MAX_NAME_LENGTH }),
    };
  }
  if (viewNames.includes(name.trim())) {
    return {
      isValid: false,
      message: i18nService.t('view_name_already_exists'),
    };
  }
  return {
    isValid: true,
    message: '',
  };
};
