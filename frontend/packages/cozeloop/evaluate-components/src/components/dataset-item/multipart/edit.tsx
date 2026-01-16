// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/use-error-in-catch */
/* eslint-disable @coze-arch/max-line-per-function */
import React, { useRef, useState, useEffect } from 'react';

import Sortable from 'sortablejs';
import { nanoid } from 'nanoid';
import cs from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { EVALUATE_MULTIPART_DATA_ABILITY_CONFIG } from '@cozeloop/evaluate-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { uploadFile } from '@cozeloop/biz-components-adapter';
import {
  ContentType,
  type Image as ImageProps,
} from '@cozeloop/api-schema/evaluation';
import { StorageProvider } from '@cozeloop/api-schema/data';
import { IconCozPlus, IconCozHandle } from '@coze-arch/coze-design/icons';
import {
  Button,
  Dropdown,
  IconButton,
  Toast,
  Typography,
  Upload,
  type UploadProps,
} from '@coze-arch/coze-design';

import { UrlInputModal } from './components/url-input-modal';
import { ICON_ID } from './components/text-item-render';
import { MultipartItemRenderer } from './components/multipart-item-renderer';
import { getMultipartConfig } from '../util';
import {
  ImageStatus,
  type DatasetItemProps,
  type MultipartItem,
} from '../type';

import styles from './index.module.less';

export const MultipartDatasetItemEdit: React.FC<DatasetItemProps> = ({
  fieldContent,
  onChange,
  className,
  multipartConfig = {},
}) => {
  const uploadRef = useRef<Upload>(null);
  const { maxFileCount, maxPartCount, maxFileSize, supportedFormats } =
    getMultipartConfig(multipartConfig);
  const sortableContainer = useRef<HTMLDivElement>(null);
  const [items, setItems] = useState<MultipartItem[]>(
    (fieldContent?.multi_part || []).map(item => ({
      ...item,
      uid: nanoid(),
    })),
  );
  const [showUrlModal, setShowUrlModal] = useState(false);
  const { spaceID } = useSpace();
  // 同步数据到父组件
  useEffect(() => {
    onChange?.({
      ...fieldContent,
      multi_part: items?.map(({ uid, sourceImage, ...item }) => item),
    });
  }, [items]);

  // 受控父数据变化
  useEffect(() => {
    if (!fieldContent?.multi_part && items?.length !== 0) {
      setItems(
        (fieldContent?.multi_part || []).map(item => ({
          ...item,
          uid: nanoid(),
        })),
      );
    }
  }, [fieldContent?.multi_part]);

  const imageCount = items?.filter(
    item => item.content_type === ContentType.Image,
  ).length;
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
          const container =
            dragElClone.getElementsByClassName('semi-collapse')?.[0];
          if (container) {
            container.setAttribute('style', 'width: 200px; overflow: hidden;');
          }
          const icon = dragElClone.getElementsByClassName(ICON_ID)?.[0];
          if (icon) {
            icon.setAttribute('style', 'visibility: visible');
          }
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
      // 添加loading状态的item
      setItems(prev => [
        ...prev,
        {
          uid,
          content_type: ContentType.Image,
          sourceImage: {
            status: ImageStatus.Loading,
            file: fileInstance,
          },
          image: {
            name: file.name,
            url,
            storage_provider: StorageProvider.ImageX,
          },
        },
      ]);
      const uri = await uploadFile({
        file: fileInstance,
        fileType: 'image',
        onProgress,
        onSuccess,
        onError,
        spaceID,
      });

      // 更新为成功状态
      setItems(prev =>
        prev.map(item =>
          item.uid === uid
            ? {
                ...item,
                sourceImage: {
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
            : item,
        ),
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
    setTimeout(() => {
      uploadRef.current?.openFileDialog();
    }, 0);
  };

  // 添加图片链接节点
  const handleAddImageUrl = () => {
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

  const dropdownMenu = (
    <Dropdown.Menu>
      {EVALUATE_MULTIPART_DATA_ABILITY_CONFIG.textEnabled ? (
        <Dropdown.Item
          onClick={handleAddText}
          className="!pl-2"
          disabled={items?.length >= maxPartCount}
        >
          {I18n.t('text')}
        </Dropdown.Item>
      ) : null}
      {EVALUATE_MULTIPART_DATA_ABILITY_CONFIG.imageFileEnabled ? (
        <Dropdown.Item
          onClick={handleAddImageFile}
          className="!pl-2"
          disabled={imageCount >= maxFileCount}
        >
          {I18n.t('image_source_file')}
        </Dropdown.Item>
      ) : null}
      {EVALUATE_MULTIPART_DATA_ABILITY_CONFIG.imageUrlEnabled ? (
        <Dropdown.Item
          className="!pl-2"
          onClick={handleAddImageUrl}
          disabled={imageCount >= maxFileCount}
        >
          {I18n.t('image_external_link')}
        </Dropdown.Item>
      ) : null}
    </Dropdown.Menu>
  );

  return (
    <div
      className={cs(
        'flex flex-col bg-[#F7F7FCA6] p-3 pr-1 rounded-[6px] max-h-[713px] overflow-auto styled-scrollbar',
        styles.container,
      )}
    >
      {/* 可拖拽容器 */}
      <div ref={sortableContainer} className="flex  flex-wrap gap-3">
        {items?.map((item, index) => (
          <div
            key={item.uid}
            className="flex items-center gap-2"
            style={{
              width: item.content_type === ContentType.Image ? '104px' : '100%',
            }}
          >
            <IconButton
              icon={<IconCozHandle className="drag-handle  coz-fg-primary" />}
              className="!w-[16px] !min-w-[16px] !p-0 !h-[24px] !rounded-[4px] "
            />

            <div className="flex-1">
              <MultipartItemRenderer
                item={item}
                onChange={newItem => handleItemChange(newItem)}
                onRemove={() => handleItemRemove(index)}
              />
            </div>
          </div>
        ))}
      </div>
      {/* 添加按钮 */}
      <Dropdown render={dropdownMenu}>
        <Button
          icon={<IconCozPlus />}
          size="small"
          className={`!w-fit ${items?.length ? 'mt-3' : ''}`}
          color="primary"
          // disabled={items.length >= maxPartCount}
        >
          {I18n.t('add_data')}
          <Typography.Text className="ml-1" type="secondary">
            {`${items.length}/${maxPartCount}`}
          </Typography.Text>
        </Button>
      </Dropdown>
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
      />

      {/* 外链输入模态框 */}
      {showUrlModal ? (
        <UrlInputModal
          visible={showUrlModal}
          maxCount={maxFileCount - imageCount}
          onConfirm={handleConfirmImageUrl}
          onCancel={() => setShowUrlModal(false)}
        />
      ) : null}
    </div>
  );
};
