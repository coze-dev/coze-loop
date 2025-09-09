// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export { TagsCreatePage } from './pages/tags-create-page';
export { TagsDetailPage } from './pages/tags-detail-page';
export { TagsListPage } from './pages/tags-list-page';

export { TagsList } from './components/tags-list';

// manual annotation
export { AnnotationPanel } from './components/annotation-panel';
export { TagSelect } from './components/tag-select';

// 新增的标签选择组件
export { TagsSelect } from './components/tags-select';
export { TagsItem } from './components/tags-item';

export { useBatchGetTags } from './hooks/use-batch-get-tags';
export { useTagFormModal } from './hooks/use-tag-form-modal';

// 导出开源接口，提供给其他模块使用
export type {
  CreateTagRequest,
  CreateTagResponse,
  UpdateTagRequest,
  UpdateTagResponse,
  SearchTagsRequest,
  SearchTagsResponse,
  GetTagDetailRequest,
  GetTagDetailResponse,
  BatchUpdateTagStatusRequest,
  BatchUpdateTagStatusResponse,
  ArchiveOptionTagRequest,
  ArchiveOptionTagResponse,
  BatchGetTagsRequest,
  BatchGetTagsResponse,
  GetTagSpecRequest,
  GetTagSpecResponse,
} from './const/api';
export { openSourceTagApi, tag } from './const/api';

export { TAG_TYPE_TO_NAME_MAP } from './const';
