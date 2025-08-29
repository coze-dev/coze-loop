/* eslint-disable max-lines-per-function */
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
import { useRef, useState } from 'react';

import cs from 'classnames';
import { useRequest } from 'ahooks';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import { InfoTooltip } from '@cozeloop/components';
import {
  useSpace,
  useDataImportApi,
  useDatasetTemplateDownload,
  FILE_FORMAT_MAP,
} from '@cozeloop/biz-hooks-adapter';
import { uploadFile } from '@cozeloop/biz-components-adapter';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { StorageProvider, FileFormat } from '@cozeloop/api-schema/data';
import { IconCozFileCsv } from '@coze-arch/coze-design/illustrations';
import { IconCozDownload, IconCozUpload } from '@coze-arch/coze-design/icons';
import {
  Button,
  Dropdown,
  // Button,
  Form,
  type FormApi,
  Modal,
  Loading,
  Typography,
  type UploadProps,
  withField,
} from '@coze-arch/coze-design';

import { getFileType, getFileHeaders } from '../../utils/upload';
import { getDefaultColumnMap } from '../../utils/import-file';
import { downloadWithUrl } from '../../utils/download-template';
import { useDatasetImportProgress } from './use-import-progress';
import { OverWriteField } from './overwrite-field';
import { ColumnMapField } from './column-map-field';

import styles from './index.module.less';
const FormColumnMapField = withField(ColumnMapField);
const FormOverWriteField = withField(OverWriteField);
export const DatasetImportItemsModal = ({
  onCancel,
  onOk,
  datasetDetail,
}: {
  onCancel: () => void;
  onOk: () => void;
  datasetDetail?: EvaluationSet;
}) => {
  const formRef = useRef<FormApi>();
  const { spaceID } = useSpace();
  const { importDataApi } = useDataImportApi();
  const [csvHeaders, setCsvHeaders] = useState<string[]>([]);
  const { startProgressTask, node } = useDatasetImportProgress(onOk);
  const [visible, setVisible] = useState(true);
  const [loading, setLoading] = useState(false);
  const { getDatasetTemplate } = useDatasetTemplateDownload();
  const guard = useGuard({ point: GuardPoint['eval.dataset.import'] });
  const dragSubTextRef = useRef<HTMLDivElement>(null);
  const [downloadingTemplateLoading, setDownloadingTemplateLoading] =
    useState<boolean>(false);
  const handleUploadFile: UploadProps['customRequest'] = async ({
    fileInstance,
    file,
    onProgress,
    onSuccess,
    onError,
  }) => {
    await uploadFile({
      file: fileInstance,
      fileType: fileInstance.type?.includes('image') ? 'image' : 'object',
      onProgress,
      onSuccess,
      onError,
      spaceID,
    });
    const fileType = getFileType(fileInstance?.name);
    formRef?.current?.setValue('fileType', fileType);
    const { headers, error } = await getFileHeaders(fileInstance);
    if (error) {
      formRef?.current?.setError('file', error);
    }
    if (headers) {
      setCsvHeaders(headers);
      formRef?.current?.setValue(
        'fieldMappings',
        getDefaultColumnMap(datasetDetail, headers),
      );
    }
  };
  const { data: templateUrlList } = useRequest(
    async () => {
      const res = await getDatasetTemplate({
        spaceID,
        datasetID: datasetDetail?.id as string,
      });
      return res?.map(item => ({
        label: `${FILE_FORMAT_MAP[item?.format || FileFormat.CSV]} 模板`,
        value: item.url,
      }));
    },
    {
      refreshDeps: [],
    },
  );
  const onSubmit = async values => {
    setLoading(true);
    try {
      const res = await importDataApi({
        workspace_id: spaceID,
        dataset_id: datasetDetail?.id as string,
        file: {
          provider: StorageProvider.S3,
          path: values.file?.[0]?.response?.Uri,
          ...(values?.fileType === FileFormat.ZIP
            ? {
                compress_format: FileFormat.ZIP,
                format: FileFormat.CSV,
              }
            : {
                format: values.fileType || FileFormat.CSV,
              }),
        },
        field_mappings: values.fieldMappings?.filter(item => !!item?.source),
        option: {
          overwrite_dataset: values.overwrite,
        },
      });
      if (res.job_id) {
        startProgressTask(res.job_id);
        setVisible(false);
      }
    } finally {
      setLoading(false);
    }
  };
  return (
    <>
      <Modal
        title="导入数据"
        width={640}
        visible={visible}
        keepDOM={true}
        onCancel={onCancel}
        className={styles.modal}
        hasScroll={false}
        footer={null}
      >
        <Form
          initValues={{
            fieldMappings: getDefaultColumnMap(datasetDetail, csvHeaders),
            overwrite: false,
            fileType: '',
          }}
          getFormApi={formApi => {
            formRef.current = formApi;
          }}
          onValueChange={values => {
            console.log('values', values);
          }}
          onSubmit={onSubmit}
        >
          {({ formState, formApi }) => {
            const file = formState.values?.file;
            const fieldMappings = formState.values?.fieldMappings;
            const disableImport =
              !file?.[0]?.response?.Uri ||
              fieldMappings?.every(item => !item?.source);
            return (
              <>
                <div
                  className={cs(styles.form, 'styled-scrollbar relative')}
                  ref={dragSubTextRef}
                >
                  <Form.Upload
                    field="file"
                    label="上传数据"
                    limit={1}
                    onChange={({ fileList }) => {
                      if (fileList.length === 0) {
                        setCsvHeaders([]);
                        formRef?.current?.setValue(
                          'fieldMappings',
                          getDefaultColumnMap(datasetDetail, []),
                        );
                      }
                    }}
                    draggable={true}
                    previewFile={() => (
                      <IconCozFileCsv className="w-[32px] h-[32px]" />
                    )}
                    className={styles.upload}
                    dragIcon={<IconCozUpload className="w-[32px] h-[32px]" />}
                    dragMainText="点击上传或者拖拽文件至此处"
                    dragSubText={
                      <div className="relative flex items-center">
                        <Typography.Text
                          className="!coz-fg-secondary"
                          size="small"
                        >
                          支持文件格式：csv、zip、xlsx、xls，文件最大200MB,
                          仅支持导入一个文件
                        </Typography.Text>
                        {templateUrlList?.length ? (
                          <Dropdown
                            getPopupContainer={() =>
                              dragSubTextRef.current || document.body
                            }
                            zIndex={100000}
                            position="bottom"
                            render={
                              <div
                                onClick={e => {
                                  e.stopPropagation();
                                }}
                              >
                                <Dropdown.Menu>
                                  {templateUrlList?.map(item => (
                                    <Dropdown.Item
                                      className="!pl-2"
                                      key={item.value}
                                      onClick={async () => {
                                        setDownloadingTemplateLoading(true);
                                        await downloadWithUrl(
                                          item.value || '',
                                          item.label,
                                        );
                                        setDownloadingTemplateLoading(false);
                                      }}
                                    >
                                      {item.label}
                                    </Dropdown.Item>
                                  ))}
                                </Dropdown.Menu>
                              </div>
                            }
                          >
                            <div
                              onClick={e => {
                                e.stopPropagation();
                              }}
                            >
                              <Typography.Text
                                link
                                icon={<IconCozDownload />}
                                className="ml-[12px]"
                                size="small"
                              >
                                下载模板
                                {downloadingTemplateLoading ? (
                                  <Loading
                                    loading
                                    size="mini"
                                    color="blue"
                                    className="w-[14px] pl-1 !h-[4px] coz-fg-primary"
                                  />
                                ) : null}
                              </Typography.Text>
                            </div>
                          </Dropdown>
                        ) : null}
                      </div>
                    }
                    action=""
                    accept=".csv, .zip, .xlsx, .xls"
                    customRequest={handleUploadFile}
                    rules={[
                      {
                        required: true,
                        message: '请上传文件',
                      },
                    ]}
                  ></Form.Upload>
                  {file?.[0]?.response?.Uri ? (
                    <Form.Slot
                      className="form-mini"
                      label={{
                        text: (
                          <div className="inline-flex items-center gap-1 !coz-fg-primary">
                            <div>列映射</div>
                            <InfoTooltip
                              className="h-[15px]"
                              content="待导入数据的列名和当前评测集列名的映射关系。"
                            />
                          </div>
                        ),
                        required: true,
                      }}
                    >
                      <Typography.Text
                        type="secondary"
                        size="small"
                        className="!coz-fg-secondary block"
                      >
                        如果待导入数据集的列没有配置映射关系，则该列不会被导入。
                      </Typography.Text>
                      {formState?.values?.fieldMappings?.map((field, index) => (
                        <FormColumnMapField
                          field={`fieldMappings[${index}]`}
                          noLabel
                          sourceColumns={csvHeaders}
                          rules={[
                            {
                              validator: (_, data, cb) => {
                                if (
                                  !data?.source &&
                                  data?.fieldSchema?.isRequired
                                ) {
                                  cb('请配置导入列');
                                  return false;
                                }
                                return true;
                              },
                            },
                          ]}
                        />
                      ))}
                    </Form.Slot>
                  ) : null}
                  <FormOverWriteField
                    field="overwrite"
                    rules={[{ required: true, message: '请选择导入方式' }]}
                    label={'导入方式'}
                  />
                </div>
                <div className="flex justify-end p-[24px] pb-0">
                  <Button
                    className="mr-2"
                    color="primary"
                    onClick={() => {
                      onCancel();
                    }}
                  >
                    取消
                  </Button>
                  <Button
                    color="brand"
                    onClick={() => {
                      formRef.current?.submitForm();
                    }}
                    loading={loading}
                    disabled={guard.data.readonly || disableImport}
                  >
                    导入
                  </Button>
                </div>
              </>
            );
          }}
        </Form>
      </Modal>
      {node}
    </>
  );
};
