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

import { useI18n } from '@/provider';

import { getErrorTypeMap, type UploadAttachmentDetail } from '../type';
import { LoopTable } from '../../table';

interface UrlInputModalProps {
  visible: boolean;
  onConfirm: (results: ImageProps[]) => void;
  onCancel: () => void;
  maxCount?: number;
  uploadImageUrl?: (
    urls: string[],
  ) => Promise<UploadAttachmentDetail[] | undefined>;
  uploadType?: 'image' | 'video';
  intranetUrlValidator?: (url: string) => boolean;
}

export const UrlInputModal: React.FC<UrlInputModalProps> = ({
  visible,
  onConfirm,
  onCancel,
  maxCount = 6,
  uploadImageUrl,
  uploadType = 'image',
  intranetUrlValidator,
}) => {
  const I18n = useI18n();
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
      setError(I18n.t('please_add_at_least_one_image_link'));
      return;
    }
    setIsUploading(true);
    setError('');
    try {
      const results = await uploadImageUrl?.(formValues?.urls);
      setUploadResults(results || []);
      setIsUploaded(true);
    } catch (err) {
      setError(I18n.t('upload_failed_please_try_again'));
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
      onSubmit={uploadImageUrl ? handleUpload : undefined}
    >
      {({ formState, formApi }) => {
        const urls: string[] = formState.values?.urls || [''];
        const canAdd = urls.length < maxCount;
        return (
          <div>
            {urls.map((_url, idx) => (
              <div key={idx} className="flex  gap-2">
                <Form.Input
                  field={`urls[${idx}]`}
                  label={{
                    text: `${uploadType === 'image' ? I18n.t('image') : I18n.t('video')}${idx + 1}`,
                    required: true,
                  }}
                  fieldClassName="flex-1"
                  placeholder={`${I18n.t('please_input_placeholder1_link_public', { placeholder1: uploadType === 'image' ? I18n.t('image') : I18n.t('video') })}`}
                  rules={[
                    {
                      validator: (_, value, cb) => {
                        if (!value) {
                          cb(
                            `${I18n.t('please_input_placeholder1_link', { placeholder1: uploadType === 'image' ? I18n.t('image') : I18n.t('video') })}`,
                          );
                          return false;
                        }
                        if (
                          intranetUrlValidator &&
                          intranetUrlValidator(value)
                        ) {
                          cb(I18n.t('please_use_public_domain'));
                          return false;
                        }
                        if (!validateUrl(value)) {
                          cb(I18n.t('please_enter_a_valid_url'));
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
              {I18n.t('space_member_role_type_add_btn')}
              <Typography.Text className="ml-1" type="secondary">
                {`${urls.length}/${maxCount}`}
              </Typography.Text>
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
      title: I18n.t('image_address'),
      dataIndex: 'originImage.url',
      width: 220,
      ellipsis: { showTitle: true },
    },
    {
      title: I18n.t('image_preview'),
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
      title: I18n.t('status'),
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
            {record?.errorType ? I18n.t('failure') : I18n.t('success')}
          </Tag>
          <Typography.Text type="secondary" className="ml-1">
            {record.errorType ? getErrorTypeMap(I18n)[record.errorType] : ''}
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
      return I18n.t('global_btn_confirm');
    }
    if (isUploading) {
      return I18n.t('cozeloop_open_evaluate_uploading');
    }
    if (isUploaded) {
      return `${I18n.t('import')}${uploadType === 'image' ? I18n.t('image') : I18n.t('video')}`;
    }
    return I18n.t('upload');
  };

  return (
    <Modal
      title={`${I18n.t('add_placeholder1_link', { placeholder1: uploadType === 'image' ? I18n.t('image') : I18n.t('video') })}`}
      visible={visible}
      onCancel={handleCancel}
      width={640}
      footer={
        <div className="flex justify-end">
          <Button onClick={handleCancel} color="primary">
            {I18n.t('cancel')}
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
          <span className="ml-2">
            {I18n.t('cozeloop_open_evaluate_uploading_image')}
          </span>
        </div>
      ) : null}

      {!isUploading && !isUploaded && renderInputStage()}
      {!isUploading && isUploaded ? renderResultStage() : null}
    </Modal>
  );
};
