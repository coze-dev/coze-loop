import { ItemErrorType } from '@cozeloop/api-schema/data';

export const DEFAULT_PAGE_SIZE = 10;
export const DATASET_ADD_ITEM_PREFIX = 'dataset-add-item';

export const ErrorTypeMap = {
  [ItemErrorType.MismatchSchema]: 'schema 不匹配',
  [ItemErrorType.EmptyData]: '空数据',
  [ItemErrorType.ExceedMaxItemSize]: '单条数据大小超限',
  [ItemErrorType.ExceedDatasetCapacity]: '数据集容量超限',
  [ItemErrorType.MalformedFile]: '文件格式错误',
  [ItemErrorType.InternalError]: '系统错误',
  [ItemErrorType.IllegalContent]: '包含非法内容',
  [ItemErrorType.MissingRequiredField]: '缺少必填字段',
  [ItemErrorType.ExceedMaxNestedDepth]: '数据嵌套层数超限',
  [ItemErrorType.TransformItemFailed]: '数据转换失败',
  [ItemErrorType.ExceedMaxImageCount]: '图片数量超限',
  [ItemErrorType.ExceedMaxImageSize]: '图片大小超限',
  [ItemErrorType.GetImageFailed]: '图片获取失败',
  [ItemErrorType.IllegalExtension]: '文件扩展名不合法',
  [ItemErrorType.UploadImageFailed]: '上传图片失败',
};
