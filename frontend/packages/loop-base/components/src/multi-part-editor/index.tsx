// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines */
/* eslint-disable complexity */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/use-error-in-catch */
/* eslint-disable @coze-arch/max-line-per-function */
import React, { useRef, useState, useEffect, useCallback } from 'react';

import Sortable from 'sortablejs';
import { nanoid } from 'nanoid';
import classNames from 'classnames';
import {
  ContentType,
  type Image as ImageProps,
} from '@cozeloop/api-schema/evaluation';
import { StorageProvider } from '@cozeloop/api-schema/data';
import { IconCozPlus, IconCozHandle } from '@coze-arch/coze-design/icons';
import {
  Button,
  IconButton,
  Menu,
  Toast,
  Typography,
  Upload,
  type UploadProps,
} from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { TooltipWhenDisabled } from '../tooltip-when-disabled';
import { getMultipartConfig } from './utils';
import {
  ImageStatus,
  type MultipartItemContentType,
  type MultipartEditorProps,
  type MultipartItem,
} from './type';
import { UrlInputModal } from './components/url-input-modal';
import { MultipartItemRenderer } from './components/multipart-item-renderer';

import styles from './index.module.less';

export const MultipartEditor: React.FC<MultipartEditorProps> = ({
  spaceID,
  uploadFile,
  value,
  onChange,
  className,
  multipartConfig,
  uploadImageUrl,
  readonly,
  imageHidden,
  videoHidden,
  intranetUrlValidator,
}) => {
  const I18n = useI18n();
  const uploadRef = useRef<Upload>(null);
  const {
    maxFileCount,
    maxPartCount,
    maxFileSize,
    imageEnabled,
    videoEnabled,
    imageSupportedFormats,
    videoSupportedFormats,
  } = getMultipartConfig(multipartConfig);
  const sortableContainer = useRef<HTMLDivElement>(null);
  const [items, setItems] = useState<MultipartItem[]>(
    (value || []).map(item => ({
      ...item,
      uid: nanoid(),
    })),
  );
  const [showUrlModal, setShowUrlModal] = useState(false);
  const [currentUploadType, setCurrentUploadType] = useState<'image' | 'video'>(
    'image',
  );
  const [supportedFormats, setSupportedFormats] = useState<string>('');

  const imageCount = items.filter(
    item =>
      item.content_type === ContentType.Image || item.content_type === 'Video',
  ).length;

  const canUsePartLimit = maxPartCount - items.length;
  const canUseFileLimit = maxFileCount - imageCount;
  const exceedFileCount = !canUseFileLimit;

  // 处理文件上传
  const handleUploadFile: UploadProps['customRequest'] = async ({
    file,
    onProgress,
    onSuccess,
    onError,
  }) => {
    const uid = nanoid();

    try {
      const fileInstance = (file.fileInstance || file) as File;
      const url = URL.createObjectURL(fileInstance);
      const beforUploadItem =
        currentUploadType === 'image'
          ? {
              sourceImage: {
                status: ImageStatus.Loading,
                file: fileInstance,
              },
              image: {
                name: file.name,
                url,
                storage_provider: StorageProvider.ImageX,
              },
            }
          : {
              sourceVideo: {
                status: ImageStatus.Loading,
                file: fileInstance,
              },
              video: {
                name: file.name,
                url,
                storage_provider: StorageProvider.ImageX,
              },
            };
      // 添加loading状态的item
      setItems(prev => [
        ...prev,
        {
          uid,
          content_type:
            currentUploadType === 'image' ? ContentType.Image : 'Video',
          ...beforUploadItem,
        } as any,
      ]);
      const uri = await uploadFile?.({
        file: fileInstance,
        fileType: 'image',
        onProgress,
        onSuccess,
        onError,
        spaceID,
      });

      // 更新为成功状态
      setItems(prev =>
        prev.map(item => {
          if (item.uid === uid) {
            const afterUploadItem =
              currentUploadType === 'image'
                ? {
                    sourceImage: {
                      ...item.sourceImage,
                      status: ImageStatus.Success,
                      file: fileInstance,
                    },
                    image: {
                      ...item.image,
                      url,
                      uri,
                      storage_provider: StorageProvider.ImageX,
                    },
                  }
                : {
                    sourceVideo: {
                      ...item.sourceVideo,
                      status: ImageStatus.Success,
                      file: fileInstance,
                    },
                    video: {
                      ...item.video,
                      url,
                      uri,
                      storage_provider: StorageProvider.ImageX,
                    },
                  };
            return {
              ...item,
              ...afterUploadItem,
            };
          }
          return item;
        }),
      );
    } catch (error) {
      // 更新为错误状态
      setItems(prev =>
        prev.map(item =>
          item.uid === uid
            ? {
                ...item,
                sourceImage: {
                  ...item.sourceImage,
                  status: ImageStatus.Error,
                },
              }
            : item,
        ),
      );
    }
  };

  // 添加文本节点
  const handleAddText = () => {
    setItems(prev => [
      ...prev,
      {
        uid: nanoid(),
        content_type: ContentType.Text,
        text: '',
      },
    ]);
  };

  // 添加图片文件节点
  const handleAddImageFile = () => {
    setCurrentUploadType('image');
    setSupportedFormats(imageSupportedFormats);
    setTimeout(() => {
      uploadRef.current?.openFileDialog();
    }, 0);
  };

  // 添加视频文件节点
  const handleAddVideoFile = () => {
    setCurrentUploadType('video');
    setSupportedFormats(videoSupportedFormats);
    setTimeout(() => {
      uploadRef.current?.openFileDialog();
    }, 0);
  };

  // 添加图片链接节点
  const handleAddImageUrl = () => {
    setCurrentUploadType('image');
    setShowUrlModal(true);
  };

  // 添加视频链接节点
  const handleAddVideoUrl = () => {
    setCurrentUploadType('video');
    setShowUrlModal(true);
  };

  // 确认添加图片链接
  const handleConfirmImageUrl = (results: ImageProps[]) => {
    const newItems = results.map(result => ({
      uid: nanoid(),
      content_type: ContentType.Image,
      image: {
        ...result,
        storage_provider: StorageProvider.ImageX,
      },
    }));

    setItems(prev => [...prev, ...newItems]);
    setShowUrlModal(false);
  };

  // 确认添加视频链接
  const handleConfirmVideoUrl = (results: ImageProps[]) => {
    const newItems = results.map(result => ({
      uid: nanoid(),
      content_type: 'Video' as MultipartItemContentType,
      video: {
        ...result,
        storage_provider: StorageProvider.ImageX,
      },
    }));

    setItems(prev => [...prev, ...newItems]);
    setShowUrlModal(false);
  };

  // 更新item
  const handleItemChange = (newItem: MultipartItem) => {
    setItems(prev =>
      prev.map(item => (item.uid === newItem.uid ? newItem : item)),
    );
  };

  // 删除item
  const handleItemRemove = (index: number) => {
    setItems(prev => prev.filter((_, i) => i !== index));
  };

  const getDisabledTooltip = useCallback(
    ({
      type,
      exceed,
      disabled,
    }: {
      type: MultipartItemContentType;
      exceed?: boolean;
      disabled?: boolean;
    }) => {
      if (type === ContentType.Image) {
        if (exceed) {
          return `${I18n.t('multi_modal_image_limit_reached', { canUseFileLimit })}`;
        }
        if (disabled) {
          return I18n.t('model_not_support_multi_modal_image');
        }
      } else if (type === 'Video') {
        if (exceed) {
          return `${I18n.t('multi_modal_video_limit_reached', { canUseFileLimit })}`;
        }
        if (disabled) {
          return I18n.t('model_not_support_multi_modal_video');
        }
      } else {
        return I18n.t('model_not_support_multi_modal');
      }
    },
    [],
  );

  const renderImageSubMenu = useCallback(
    (children: React.ReactNode) => (
      <TooltipWhenDisabled
        disabled={exceedFileCount || !imageEnabled}
        content={getDisabledTooltip({
          type: ContentType.Image,
          exceed: exceedFileCount,
          disabled: !imageEnabled,
        })}
        theme="dark"
        needWrap
      >
        {children}
      </TooltipWhenDisabled>
    ),

    [exceedFileCount, imageEnabled],
  );

  const renderVideoSubMenu = useCallback(
    (children: React.ReactNode) => (
      <TooltipWhenDisabled
        disabled={exceedFileCount || !videoEnabled}
        content={getDisabledTooltip({
          type: 'Video',
          exceed: exceedFileCount,
          disabled: !videoEnabled,
        })}
        theme="dark"
        needWrap
      >
        {children}
      </TooltipWhenDisabled>
    ),

    [exceedFileCount, videoEnabled],
  );

  const dropdownMenu = (
    <Menu.SubMenu mode="menu">
      <Menu.Item onClick={handleAddText} disabled={imageCount >= maxPartCount}>
        {I18n.t('text')}
      </Menu.Item>

      {imageHidden ? null : (
        <Menu.Item
          onClick={handleAddImageFile}
          disabled={exceedFileCount || !imageEnabled}
        >
          {renderImageSubMenu(
            <span className="w-full">{I18n.t('image_source_file')}</span>,
          )}
        </Menu.Item>
      )}

      {imageHidden ? null : (
        <Menu.Item
          onClick={handleAddImageUrl}
          disabled={exceedFileCount || !imageEnabled}
        >
          {renderImageSubMenu(
            <span className="w-full">{I18n.t('image_external_link')}</span>,
          )}
        </Menu.Item>
      )}

      {videoHidden ? null : (
        <Menu.Item
          onClick={handleAddVideoFile}
          disabled={exceedFileCount || !videoEnabled}
        >
          {renderVideoSubMenu(
            <span className="w-full">{I18n.t('video_source_file')}</span>,
          )}
        </Menu.Item>
      )}

      {videoHidden ? null : (
        <Menu.Item
          onClick={handleAddVideoUrl}
          disabled={exceedFileCount || !videoEnabled}
        >
          {renderVideoSubMenu(
            <span className="w-full">{I18n.t('video_external_link')}</span>,
          )}
        </Menu.Item>
      )}
    </Menu.SubMenu>
  );

  // 同步数据到父组件
  useEffect(() => {
    onChange?.(items);
  }, [items]);

  // 初始化sortablejs拖拽排序
  useEffect(() => {
    if (sortableContainer.current) {
      new Sortable(sortableContainer.current, {
        animation: 150,
        handle: '.drag-handle',
        ghostClass: styles.ghost,
        onEnd: evt => {
          setItems(list => {
            const draft = [...(list ?? [])];
            if (draft.length) {
              const { oldIndex = 0, newIndex = 0 } = evt;
              const [item] = draft.splice(oldIndex, 1);
              draft.splice(newIndex, 0, item);
            }
            return draft;
          });
        },
        setData(dataTransfer, dragEl) {
          // dragEl 是被拖拽的元素
          // dataTransfer 是拖拽数据传输对象
          // 创建自定义预览元素
          // 浅复制（只复制元素本身，不包含子元素）

          // 深复制（复制元素及其所有子元素）
          const dragElClone: HTMLElement = dragEl.cloneNode(
            true,
          ) as HTMLElement;
          const customPreview = document.createElement('div');
          // // 临时添加到DOM（必须在可见区域外）
          customPreview.style.position = 'absolute';
          customPreview.style.top = '-1000px';
          customPreview.style.width = '200px';
          customPreview.appendChild(dragElClone);
          const wrapper = dragElClone.getElementsByClassName(
            'semi-collapsible-wrapper',
          )?.[0];
          if (wrapper) {
            wrapper.setAttribute(
              'style',
              'height: 0px; width: 0px; overflow: hidden;',
            );
          }
          document.body.appendChild(customPreview);
          dataTransfer.setDragImage(wrapper ? customPreview : dragEl, 0, 0);
          // 清理临时元素
          setTimeout(() => {
            if (customPreview.parentNode) {
              document.body.removeChild(customPreview);
            }
          }, 0);
        },
      });
    }
  }, []);

  return (
    <div
      className={classNames(
        'flex flex-col gap-2 p-0 max-h-[713px] overflow-auto styled-scrollbar',
        className,
      )}
    >
      {/* 可拖拽容器 */}
      <div
        ref={sortableContainer}
        className={classNames(
          'flex flex-wrap gap-2 rounded-[6px] coz-bg-primary p-2',
          {
            hidden: !items.length,
          },
        )}
      >
        {items.map((item, index) => (
          <div key={item.uid} className="flex items-center gap-2 w-full">
            {readonly ? null : (
              <IconButton
                icon={<IconCozHandle className="drag-handle" />}
                color="secondary"
              />
            )}
            <div className="flex-1">
              <MultipartItemRenderer
                item={item}
                onChange={newItem => handleItemChange(newItem)}
                onRemove={() => handleItemRemove(index)}
                readonly={readonly}
              />
            </div>
          </div>
        ))}
      </div>
      {/* 添加按钮 */}
      {items.length >= maxPartCount || readonly ? (
        <Button
          icon={<IconCozPlus />}
          size="small"
          className="!w-fit"
          color="primary"
          disabled
        >
          {I18n.t('add_data')}
          <Typography.Text className="ml-1 !text-inherit" type="secondary">
            {`${items.length}/${maxPartCount}`}
          </Typography.Text>
        </Button>
      ) : (
        <Menu
          trigger="click"
          clickToHide
          render={dropdownMenu}
          position="bottomLeft"
        >
          <Button
            icon={<IconCozPlus />}
            size="small"
            className="!w-fit"
            color="primary"
            disabled={items.length >= maxPartCount}
          >
            {I18n.t('add_data')}
            <Typography.Text className="ml-1" type="secondary">
              {`${items.length}/${maxPartCount}`}
            </Typography.Text>
          </Button>
        </Menu>
      )}
      {/* 隐藏的文件上传组件 */}
      <Upload
        ref={uploadRef}
        action=""
        maxSize={maxFileSize}
        onSizeError={() => {
          Toast.error(I18n.t('cozeloop_open_evaluate_image_size_limit_20mb'));
        }}
        accept={supportedFormats}
        customRequest={handleUploadFile}
        showUploadList={false}
        style={{ display: 'none' }}
        multiple
        limit={
          canUseFileLimit > canUsePartLimit ? canUsePartLimit : canUseFileLimit
        }
        onExceed={() => {
          Toast.error(I18n.t('image_or_node_quantity_limit'));
        }}
      />

      {/* 外链输入模态框 */}
      {showUrlModal ? (
        <UrlInputModal
          visible={showUrlModal}
          maxCount={
            canUseFileLimit > canUsePartLimit
              ? canUsePartLimit
              : canUseFileLimit
          }
          onConfirm={
            currentUploadType === 'image'
              ? handleConfirmImageUrl
              : handleConfirmVideoUrl
          }
          onCancel={() => setShowUrlModal(false)}
          uploadImageUrl={uploadImageUrl}
          uploadType={currentUploadType}
          intranetUrlValidator={intranetUrlValidator}
        />
      ) : null}
    </div>
  );
};
