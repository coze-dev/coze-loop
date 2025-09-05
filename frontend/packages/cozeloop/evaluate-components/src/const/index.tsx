import { I18n } from '@cozeloop/i18n-adapter';
import { ItemErrorType } from '@cozeloop/api-schema/data';

export const DEFAULT_PAGE_SIZE = 10;
export const DATASET_ADD_ITEM_PREFIX = 'dataset-add-item';

export const ErrorTypeMap = {
  [ItemErrorType.MismatchSchema]: I18n.t('schema_mismatch'),
  [ItemErrorType.EmptyData]: I18n.t('empty_data'),
  [ItemErrorType.ExceedMaxItemSize]: I18n.t('single_data_size_exceeded'),
  [ItemErrorType.ExceedDatasetCapacity]: I18n.t('dataset_capacity_exceeded'),
  [ItemErrorType.MalformedFile]: I18n.t('file_format_error'),
  [ItemErrorType.InternalError]: I18n.t('system_error'),
  [ItemErrorType.IllegalContent]: I18n.t('contains_illegal_content'),
  [ItemErrorType.MissingRequiredField]: I18n.t('missing_required_field'),
  [ItemErrorType.ExceedMaxNestedDepth]: I18n.t(
    'data_engine_data_nesting_exceeds_limit',
  ),
  [ItemErrorType.TransformItemFailed]: I18n.t(
    'data_engine_data_conversion_failed',
  ),
  [ItemErrorType.ExceedMaxImageCount]: I18n.t(
    'data_engine_exceed_max_image_count',
  ),
  [ItemErrorType.ExceedMaxImageSize]: I18n.t(
    'data_engine_exceed_max_image_size',
  ),
  [ItemErrorType.GetImageFailed]: I18n.t('data_engine_get_image_failed'),
  [ItemErrorType.IllegalExtension]: I18n.t('data_engine_illegal_extension'),
  [ItemErrorType.UploadImageFailed]: I18n.t(
    'cozeloop_open_evaluate_image_upload_failed',
  ),
};
