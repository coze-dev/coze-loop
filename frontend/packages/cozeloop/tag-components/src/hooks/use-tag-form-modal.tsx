// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useState, useCallback, useRef, useMemo } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type tag } from '@cozeloop/api-schema/data';
import { IconCozInfoCircleFillPalette } from '@coze-arch/coze-design/icons';
import { Banner, Modal, Toast } from '@coze-arch/coze-design';

import {
  TagsForm,
  type TagFormRef,
  type FormValues,
} from '@/components/tags-form';

import { useUpdateTag } from './use-update-tag';
import { useCreateTag } from './use-create-tag';

/**
 * 标签表单弹窗配置选项
 */
export interface UseTagFormModalOptions {
  /** 确认操作成功时的回调函数（创建或更新标签成功后触发） */
  onSuccess?: () => void;
  /** 取消操作时的回调函数（点击取消按钮时触发） */
  onCancel?: () => void;
  /** 弹窗关闭时的回调函数（任何方式关闭弹窗都会触发） */
  onClose?: () => void;
}

/**
 * 弹窗回调函数配置
 */
export interface ModalCallbacks {
  /** 确认操作成功时的回调函数（创建或更新标签成功后触发） */
  onSuccess?: () => void;
  /** 取消操作时的回调函数（点击取消按钮时触发） */
  onCancel?: () => void;
  /** 弹窗关闭时的回调函数（任何方式关闭弹窗都会触发） */
  onClose?: () => void;
}

/**
 * 标签表单弹窗Hook返回值
 */
export interface UseTagFormModalReturn {
  /** 打开新建标签弹窗 */
  openCreate: (callbacks?: ModalCallbacks) => void;
  /** 打开编辑标签弹窗 */
  openEdit: (tagInfo: tag.TagInfo, callbacks?: ModalCallbacks) => void;
  /** 打开只读标签弹窗 */
  openReadonly: (tagInfo: tag.TagInfo, callbacks?: ModalCallbacks) => void;
  /** 关闭弹窗 */
  close: () => void;
  /** 弹窗组件，需要在JSX中渲染 */
  modal: React.ReactElement;
}

/**
 * 标签表单弹窗Hook
 *
 * 提供标签的新建、编辑、只读查看功能，支持三种模式：
 * - create: 新建标签模式，表单为空
 * - edit: 编辑标签模式，表单预填充标签信息
 * - readonly: 只读查看模式，表单不可编辑
 *
 * @param options - 配置选项
 * @param options.onSuccess - 确认操作成功时的回调函数（创建或更新标签成功后触发）
 * @param options.onCancel - 取消操作时的回调函数（点击取消按钮时触发）
 * @param options.onClose - 弹窗关闭时的回调函数（任何方式关闭弹窗都会触发）
 * @returns 返回弹窗控制方法和弹窗组件
 *
 * @example
 * ```tsx
 * // 方式1：使用默认回调（向后兼容）
 * const { openCreate, openEdit, openReadonly, modal } = useTagFormModal({
 *   onSuccess: () => console.log('操作成功'),
 *   onCancel: () => console.log('取消操作'),
 *   onClose: () => console.log('弹窗关闭'),
 * });
 *
 * // 方式2：动态传入回调（推荐）
 * const { openCreate, openEdit, openReadonly, modal } = useTagFormModal();
 *
 * const handleCreate = () => {
 *   openCreate({
 *     onSuccess: () => console.log('创建成功'),
 *     onCancel: () => console.log('取消创建'),
 *   });
 * };
 *
 * const handleEdit = (tagInfo) => {
 *   openEdit(tagInfo, {
 *     onSuccess: () => console.log('编辑成功'),
 *     onCancel: () => console.log('取消编辑'),
 *   });
 * };
 *
 * return (
 *   <div>
 *     <Button onClick={handleCreate}>新建标签</Button>
 *     <Button onClick={() => handleEdit(tagInfo)}>编辑标签</Button>
 *     {modal}
 *   </div>
 * );
 * ```
 */
export const useTagFormModal = (
  options: UseTagFormModalOptions = {},
): UseTagFormModalReturn => {
  const [visible, setVisible] = useState(false);
  const [mode, setMode] = useState<'create' | 'edit' | 'readonly'>('create');
  const [defaultValues, setDefaultValues] = useState<tag.TagInfo | undefined>();
  const [currentCallbacks, setCurrentCallbacks] = useState<ModalCallbacks>({});

  const formRef = useRef<TagFormRef>(null);

  const createTagMutation = useCreateTag();
  const updateTagMutation = useUpdateTag();

  const loading = createTagMutation.loading || updateTagMutation.loading;

  // 获取当前回调函数，优先使用动态回调，否则使用默认回调
  const getCurrentCallback = useCallback(
    (key: keyof ModalCallbacks) => currentCallbacks[key] || options[key],
    [currentCallbacks, options],
  );

  /**
   * 通用打开弹窗方法
   */
  const openModal = useCallback(
    (
      modalMode: 'create' | 'edit' | 'readonly',
      tagInfo?: tag.TagInfo,
      callbacks?: ModalCallbacks,
    ) => {
      setMode(modalMode);
      setDefaultValues(tagInfo);
      setCurrentCallbacks(callbacks || {});
      setVisible(true);
    },
    [],
  );

  /**
   * 打开新建标签弹窗
   *
   * 设置模式为create，清空表单数据，显示弹窗
   * @param callbacks - 可选的回调函数，会覆盖默认回调
   */
  const openCreate = useCallback(
    (callbacks?: ModalCallbacks) => {
      openModal('create', undefined, callbacks);
    },
    [openModal],
  );

  /**
   * 打开编辑标签弹窗
   *
   * @param tagInfo - 要编辑的标签信息
   * @param callbacks - 可选的回调函数，会覆盖默认回调
   */
  const openEdit = useCallback(
    (tagInfo: tag.TagInfo, callbacks?: ModalCallbacks) => {
      openModal('edit', tagInfo, callbacks);
    },
    [openModal],
  );

  /**
   * 打开只读标签弹窗
   *
   * @param tagInfo - 要查看的标签信息
   * @param callbacks - 可选的回调函数，会覆盖默认回调
   */
  const openReadonly = useCallback(
    (tagInfo: tag.TagInfo, callbacks?: ModalCallbacks) => {
      openModal('readonly', tagInfo, callbacks);
    },
    [openModal],
  );

  /**
   * 关闭弹窗
   *
   * 隐藏弹窗，清空表单数据，触发onClose回调
   */
  const close = useCallback(() => {
    setVisible(false);
    setDefaultValues(undefined);
    getCurrentCallback('onClose')?.();
  }, [getCurrentCallback]);

  /**
   * 处理取消操作
   *
   * 触发onCancel回调，然后关闭弹窗
   */
  const handleCancel = useCallback(() => {
    getCurrentCallback('onCancel')?.();
    close();
  }, [getCurrentCallback, close]);

  /**
   * 处理表单提交
   *
   * 根据当前模式执行创建或更新操作
   * @param values - 表单数据
   */
  const handleSubmit = useCallback(
    async (values: FormValues) => {
      try {
        if (mode === 'create') {
          await createTagMutation.runAsync(values);
          Toast.success(I18n.t('tag_create_success'));
        } else if (mode === 'edit') {
          await updateTagMutation.runAsync(values);
          Toast.success(I18n.t('tag_update_success'));
        }

        getCurrentCallback('onSuccess')?.();
        close();
      } catch (error) {
        console.error('标签操作失败:', error);
        Toast.error(
          mode === 'create'
            ? I18n.t('tag_create_failure')
            : I18n.t('tag_update_failure'),
        );
      }
    },
    [mode, createTagMutation, updateTagMutation, close, getCurrentCallback],
  );

  /**
   * 处理确认按钮点击
   *
   * 触发表单提交
   */
  const handleOk = useCallback(() => {
    formRef.current?.submit();
  }, []);

  // 根据模式设置弹窗文案
  const modalConfig = useMemo(() => {
    const configs = {
      create: {
        title: I18n.t('create_tag'),
        okText: I18n.t('create'),
      },
      edit: {
        title: I18n.t('edit_tag'),
        okText: I18n.t('save'),
      },
      readonly: {
        title: I18n.t('view_tag'),
        okText: I18n.t('read_only'),
      },
    };
    return configs[mode];
  }, [mode]);

  const { title: modalTitle, okText: modalOkText } = modalConfig;

  // 弹窗组件，始终返回Modal组件，由visible属性控制显示/隐藏
  const modal = (
    <Modal
      visible={visible}
      title={modalTitle}
      onOk={handleOk}
      okText={modalOkText}
      onCancel={handleCancel}
      cancelText={I18n.t('cancel')}
      confirmLoading={loading}
      width={600}
      height={640}
      maskClosable={false}
    >
      <div className="h-full">
        <Banner
          className="coz-mg-hglt-secondary mb-4"
          justify="start"
          type="info"
          icon={<IconCozInfoCircleFillPalette className="coz-fg-hglt" />}
          description={I18n.t('changes_sync_to_space_tag_management')}
          closeIcon={null}
        />
        <TagsForm
          ref={formRef}
          entry={mode === 'create' ? 'crete-tag' : 'edit-tag'}
          defaultValues={defaultValues}
          onSubmit={handleSubmit}
        />
      </div>
    </Modal>
  );

  return {
    openCreate,
    openEdit,
    openReadonly,
    close,
    modal,
  };
};

/**
 * 使用示例：
 *
 * ```tsx
 * import { useTagFormModal } from '@/hooks/use-tag-form-modal';
 * import { Button } from '@coze-arch/coze-design';
 *
 * const MyComponent = () => {
 *   // 方式1：使用默认回调（向后兼容）
 *   const { openCreate, openEdit, openReadonly, modal } = useTagFormModal({
 *     onSuccess: () => console.log('操作成功'),
 *     onCancel: () => console.log('取消操作'),
 *     onClose: () => console.log('弹窗关闭'),
 *   });
 *
 *   // 方式2：动态传入回调（推荐）
 *   const { openCreate, openEdit, openReadonly, modal } = useTagFormModal();
 *
 *   const handleCreate = () => {
 *     openCreate({
 *       onSuccess: () => console.log('创建成功'),
 *       onCancel: () => console.log('取消创建'),
 *     });
 *   };
 *
 *   const handleEdit = (tagInfo) => {
 *     openEdit(tagInfo, {
 *       onSuccess: () => console.log('编辑成功'),
 *       onCancel: () => console.log('取消编辑'),
 *     });
 *   };
 *
 *   return (
 *     <div>
 *       <Button onClick={handleCreate}>新建标签</Button>
 *       <Button onClick={() => handleEdit(tagInfo)}>编辑标签</Button>
 *       {modal}
 *     </div>
 *   );
 * };
 * ```
 */
