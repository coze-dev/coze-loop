/**
 * @fileoverview 标签组件API统一导出模块
 * @description 提供标签相关的所有API接口和类型定义，方便其他项目使用
 * @module tag-components/api
 * @author CozeLoop Team
 * @version 1.0.0
 */

import {
  type CreateTagRequest,
  type CreateTagResponse,
  type UpdateTagRequest,
  type UpdateTagResponse,
  type SearchTagsRequest,
  type SearchTagsResponse,
  type GetTagDetailRequest,
  type GetTagDetailResponse,
  type BatchUpdateTagStatusRequest,
  type BatchUpdateTagStatusResponse,
  type ArchiveOptionTagRequest,
  type ArchiveOptionTagResponse,
  type BatchGetTagsRequest,
  type BatchGetTagsResponse,
  type GetTagSpecRequest,
  type GetTagSpecResponse,
} from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';

/**
 * 标签相关API类型定义导出
 * @description 导出所有标签相关的请求和响应类型，供其他模块使用
 */
export type {
  /** 创建标签请求参数类型 */
  CreateTagRequest,
  /** 创建标签响应数据类型 */
  CreateTagResponse,
  /** 更新标签请求参数类型 */
  UpdateTagRequest,
  /** 更新标签响应数据类型 */
  UpdateTagResponse,
  /** 搜索标签请求参数类型 */
  SearchTagsRequest,
  /** 搜索标签响应数据类型 */
  SearchTagsResponse,
  /** 获取标签详情请求参数类型 */
  GetTagDetailRequest,
  /** 获取标签详情响应数据类型 */
  GetTagDetailResponse,
  /** 批量更新标签状态请求参数类型 */
  BatchUpdateTagStatusRequest,
  /** 批量更新标签状态响应数据类型 */
  BatchUpdateTagStatusResponse,
  /** 归档选项标签请求参数类型 */
  ArchiveOptionTagRequest,
  /** 归档选项标签响应数据类型 */
  ArchiveOptionTagResponse,
  /** 批量获取标签请求参数类型 */
  BatchGetTagsRequest,
  /** 批量获取标签响应数据类型 */
  BatchGetTagsResponse,
  /** 获取标签规格请求参数类型 */
  GetTagSpecRequest,
  /** 获取标签规格响应数据类型 */
  GetTagSpecResponse,
};

/**
 * 标签数据常量导出
 * @description 导出标签相关的常量数据
 */
export { tag } from '@cozeloop/api-schema/data';

/**
 * 标签API接口对象
 * @description 提供标签相关的所有API方法，包括创建、查询、更新、删除等操作
 */
export const openSourceTagApi: {
  /**
   * 创建标签
   * @param params - 创建标签的请求参数
   * @returns Promise<CreateTagResponse> 创建标签的响应结果
   */
  createTag: (params: CreateTagRequest) => Promise<CreateTagResponse>;

  /**
   * 获取标签详情
   * @param params - 获取标签详情的请求参数
   * @returns Promise<GetTagDetailResponse> 标签详情响应结果
   */
  getTagDetail: (params: GetTagDetailRequest) => Promise<GetTagDetailResponse>;

  /**
   * 更新标签
   * @param params - 更新标签的请求参数
   * @returns Promise<UpdateTagResponse> 更新标签的响应结果
   */
  updateTag: (params: UpdateTagRequest) => Promise<UpdateTagResponse>;

  /**
   * 搜索标签列表
   * @param params - 搜索标签的请求参数
   * @returns Promise<SearchTagsResponse> 搜索标签的响应结果
   */
  searchTags: (params: SearchTagsRequest) => Promise<SearchTagsResponse>;

  /**
   * 批量更新标签状态
   * @param params - 批量更新标签状态的请求参数
   * @returns Promise<BatchUpdateTagStatusResponse> 批量更新状态的响应结果
   */
  batchUpdateTagStatus: (
    params: BatchUpdateTagStatusRequest,
  ) => Promise<BatchUpdateTagStatusResponse>;

  /**
   * 归档选项标签
   * @param params - 归档选项标签的请求参数
   * @returns Promise<ArchiveOptionTagResponse> 归档操作的响应结果
   */
  archiveOptionTag: (
    params: ArchiveOptionTagRequest,
  ) => Promise<ArchiveOptionTagResponse>;

  /**
   * 根据TagKeyID批量获取标签信息
   * @param params - 批量获取标签的请求参数
   * @returns Promise<BatchGetTagsResponse> 批量获取标签的响应结果
   */
  batchGetTags: (params: BatchGetTagsRequest) => Promise<BatchGetTagsResponse>;

  /**
   * 获取标签规格配置
   * @param params - 获取标签规格的请求参数
   * @returns Promise<GetTagSpecResponse> 标签规格配置的响应结果
   */
  getTagSpec: (params: GetTagSpecRequest) => Promise<GetTagSpecResponse>;
} = {
  // 创建标签
  createTag: DataApi.CreateTag,

  // 获取标签明细
  getTagDetail: DataApi.GetTagDetail,

  // 更新标签
  updateTag: DataApi.UpdateTag,

  // 搜索标签列表
  searchTags: DataApi.SearchTags,

  // 批量更新标签状态
  batchUpdateTagStatus: DataApi.BatchUpdateTagStatus,

  // 单选标签归档
  archiveOptionTag: DataApi.ArchiveOptionTag,

  // 根据TagKeyID获取标签信息
  batchGetTags: DataApi.BatchGetTags,

  // 获取标签配置
  getTagSpec: DataApi.GetTagSpec,
};
