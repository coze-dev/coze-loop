// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import React, { useState, useRef } from 'react';

import { type Image as ImageProps } from '@cozeloop/api-schema/evaluation';
import {
  IconCozTrashCan,
  IconCozPlus,
  IconCozCheckMarkCircleFill,
  IconCozCrossCircleFill,
} from '@coze-arch/coze-design/icons';
import {
  Modal,
  Button,
  Image,
  Spin,
  Form,
  type FormApi,
  Typography,
  Tag,
  type ColumnProps,
} from '@coze-arch/coze-design';

import { LoopTable } from '@/table';

import { ErrorTypeMap, type UploadAttachmentDetail } from '../type';

interface UrlInputModalProps {
  visible: boolean;
  onConfirm: (results: ImageProps[]) => void;
  onCancel: () => void;
  maxCount?: number;
  uploadImageUrl?: (
    urls: string[],
  ) => Promise<UploadAttachmentDetail[] | undefined>;
}

export const UrlInputModal: React.FC<UrlInputModalProps> = ({
  visible,
  onConfirm,
  onCancel,
  maxCount = 6,
  uploadImageUrl,
}) => {
  const [error, setError] = useState('');
  const [isUploading, setIsUploading] = useState(false);
  const [isUploaded, setIsUploaded] = useState(false);
  const [uploadResults, setUploadResults] = useState<UploadAttachmentDetail[]>(
    [],
  );
  const formRef = useRef<FormApi>();
  // Validate helpers
  const validateUrl = (url: string) => {
    try {
      new URL(url);
      return true;
    } catch {
      return false;
    }
  };

  // Upload logic
  const handleUpload = async formValues => {
    if (formValues?.urls?.length === 0) {
      setError('请至少添加一个图片链接');
      return;
    }
    setIsUploading(true);
    setError('');
    try {
      const results = await uploadImageUrl?.(formValues?.urls);
      setUploadResults(results || []);
      setIsUploaded(true);
    } catch (err) {
      setError('上传失败，请重试');
    } finally {
      setIsUploading(false);
    }
  };

  const handleConfirm = async () => {
    if (!uploadImageUrl) {
      const data = await formRef.current
        ?.validate()
        ?.catch(e => console.error(e));
      if (Array.isArray(data?.urls)) {
        onConfirm(
          data?.urls?.map(item => ({
            name: item,
            url: item,
            thumb_url: item,
          })),
        );
      }
      return;
    }
    if (isUploaded) {
      const successResults = uploadResults.filter(
        item => item.errorType === undefined,
      );
      onConfirm(
        successResults.map(item => ({
          name: item.image?.name,
          url: item.originImage?.url,
          uri: item.image?.uri,
          thumb_url: item.image?.thumb_url,
        })),
      );
    } else {
      await formRef.current?.submitForm();
    }
  };

  const handleCancel = () => {
    setError('');
    setIsUploading(false);
    setIsUploaded(false);
    setUploadResults([]);
    onCancel();
  };

  // Dynamic form for URLs
  const renderInputStage = () => (
    <Form
      initValues={{ urls: [''] }}
      getFormApi={api => (formRef.current = api)}
      onSubmit={handleUpload}
    >
      {({ formState, formApi }) => {
        const urls: string[] = formState.values?.urls || [''];
        const canAdd = urls.length < maxCount;
        return (
          <div>
            {urls.map((url, idx) => (
              <div key={idx} className="flex  gap-2">
                <Form.Input
                  field={`urls[${idx}]`}
                  label={{
                    text: `图片${idx + 1}`,
                    required: true,
                  }}
                  fieldClassName="flex-1"
                  placeholder="请输入图片链接"
                  rules={[
                    {
                      validator: (_, value, cb) => {
                        if (!value) {
                          cb('请输入图片链接');
                          return false;
                        }
                        if (!validateUrl(value)) {
                          cb('请输入有效的URL');
                          return false;
                        }
                        return true;
                      },
                    },
                  ]}
                  className="w-full"
                />
                <Button
                  size="small"
                  color="secondary"
                  icon={<IconCozTrashCan className="w-[14px] h-[14px]" />}
                  onClick={() => {
                    formApi.setValue(
                      'urls',
                      urls.filter((_, i) => i !== idx),
                    );
                  }}
                  className="mt-[42px]"
                  disabled={urls.length === 1}
                />
              </div>
            ))}
            <Button
              color="primary"
              disabled={!canAdd}
              icon={<IconCozPlus />}
              onClick={() => formApi.setValue('urls', [...urls, ''])}
              className="mt-2"
            >
              添加
              <Typography.Text
                className="ml-1"
                type="secondary"
              >{`${urls.length}/${maxCount}`}</Typography.Text>
            </Button>
            {error ? (
              <div className="text-red-500 text-sm mt-1">{error}</div>
            ) : null}
          </div>
        );
      }}
    </Form>
  );

  const columns: ColumnProps<UploadAttachmentDetail>[] = [
    {
      title: '图片地址',
      dataIndex: 'originImage.url',
      width: 220,
      ellipsis: { showTitle: true },
    },
    {
      title: '图片预览',
      dataIndex: 'originImage.url',
      width: 120,
      render: (url: string) => (
        <Image
          src={url}
          width={60}
          height={60}
          imgStyle={{ objectFit: 'contain' }}
        />
      ),
    },
    {
      title: '状态',
      key: 'status',
      align: 'left',
      width: 200,
      render: (record: UploadAttachmentDetail) => (
        <div className="flex items-center">
          <Tag
            prefixIcon={
              record?.errorType ? (
                <IconCozCrossCircleFill />
              ) : (
                <IconCozCheckMarkCircleFill />
              )
            }
            color={record?.errorType ? 'red' : 'green'}
          >
            {record?.errorType ? '失败' : '成功'}
          </Tag>
          <Typography.Text type="secondary" className="ml-1">
            {record.errorType ? ErrorTypeMap[record.errorType] : ''}
          </Typography.Text>
        </div>
      ),
    },
  ];

  const renderResultStage = () => (
    <div className="space-y-4">
      <LoopTable
        tableProps={{
          columns,
          dataSource: uploadResults,
          rowKey: 'id',
          pagination: false,
          size: 'small',
        }}
      />
    </div>
  );

  const getConfirmButtonText = () => {
    if (!uploadImageUrl) {
      return '确认';
    }
    if (isUploading) {
      return '上传中...';
    }
    if (isUploaded) {
      return '导入图片';
    }
    return '上传';
  };

  return (
    <Modal
      title="添加图片链接"
      visible={visible}
      onCancel={handleCancel}
      width={640}
      footer={
        <div className="flex justify-end gap-2">
          <Button onClick={handleCancel} color="primary">
            取消
          </Button>
          <Button
            type="primary"
            onClick={handleConfirm}
            loading={isUploading}
            // disabled={!isUploaded && urls.length === 0}
          >
            {getConfirmButtonText()}
          </Button>
        </div>
      }
    >
      {isUploading ? (
        <div className="flex items-center justify-center py-8">
          <Spin size="large" />
          <span className="ml-2">正在上传图片...</span>
        </div>
      ) : null}

      {!isUploading && !isUploaded && renderInputStage()}
      {!isUploading && isUploaded ? renderResultStage() : null}
    </Modal>
  );
};
