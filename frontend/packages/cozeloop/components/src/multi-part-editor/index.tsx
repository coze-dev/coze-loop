/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/use-error-in-catch */
/* eslint-disable @coze-arch/max-line-per-function */
import React, { useRef, useState, useEffect } from 'react';

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
  Dropdown,
  IconButton,
  Toast,
  Typography,
  Upload,
  type UploadProps,
} from '@coze-arch/coze-design';

import { getMultipartConfig } from './utils';
import {
  ImageStatus,
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
  multipartConfig = {},
  uploadImageUrl,
  readonly,
}) => {
  const uploadRef = useRef<Upload>(null);
  const { maxFileCount, maxPartCount, maxFileSize, supportedFormats } =
    getMultipartConfig(multipartConfig);
  const sortableContainer = useRef<HTMLDivElement>(null);
  const [items, setItems] = useState<MultipartItem[]>(
    (value || []).map(item => ({
      ...item,
      uid: nanoid(),
    })),
  );
  const [showUrlModal, setShowUrlModal] = useState(false);

  // 同步数据到父组件
  useEffect(() => {
    onChange?.(items);
  }, [items]);
  const imageCount = items.filter(
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
          console.log(evt);
          setItems(list => {
            const draft = [...(list ?? [])];
            if (draft.length) {
              const { oldIndex = 0, newIndex = 0 } = evt;
              const [item] = draft.splice(oldIndex, 1);
              draft.splice(newIndex, 0, item);
            }
            console.log(draft);
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
    console.log(newItem);
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
      <Dropdown.Item
        onClick={handleAddText}
        disabled={imageCount >= maxPartCount}
        className="w-[140px]"
      >
        文本
      </Dropdown.Item>
      <Dropdown.Item
        onClick={handleAddImageFile}
        disabled={imageCount >= maxFileCount}
        className="w-[140px]"
      >
        图片-源文件
      </Dropdown.Item>
      <Dropdown.Item
        onClick={handleAddImageUrl}
        disabled={imageCount >= maxFileCount}
        className="w-[140px]"
      >
        图片-外链
      </Dropdown.Item>
    </Dropdown.Menu>
  );

  const canUsePartLimit = maxPartCount - items.length;
  const canUseFileLimit = maxFileCount - imageCount;

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
          添加数据
          <Typography.Text
            className="ml-1"
            type="secondary"
          >{`${items.length}/${maxPartCount}`}</Typography.Text>
        </Button>
      ) : (
        <Dropdown render={dropdownMenu}>
          <Button
            icon={<IconCozPlus />}
            size="small"
            className="!w-fit"
            color="primary"
            disabled={items.length >= maxPartCount || readonly}
          >
            添加数据
            <Typography.Text
              className="ml-1"
              type="secondary"
            >{`${items.length}/${maxPartCount}`}</Typography.Text>
          </Button>
        </Dropdown>
      )}
      {/* 隐藏的文件上传组件 */}
      <Upload
        ref={uploadRef}
        action=""
        maxSize={maxFileSize}
        onSizeError={() => {
          Toast.error('图片大小不能超过20MB');
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
          Toast.error('图片数量不能超过20张或节点数量不能超过50个');
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
          onConfirm={handleConfirmImageUrl}
          onCancel={() => setShowUrlModal(false)}
          uploadImageUrl={uploadImageUrl}
        />
      ) : null}
    </div>
  );
};
