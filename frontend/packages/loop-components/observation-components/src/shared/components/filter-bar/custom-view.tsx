// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable import/order */

/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable complexity */
/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */
import { useState } from 'react';

import {
  type FieldMeta,
  PlatformType,
  SpanListType,
  type ListViewsResponse,
} from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';
import {
  IconCozTrashCan,
  IconCozTemplate,
  IconCozPlus,
  IconCozEdit,
  IconCozCheckMark,
  IconCozDocumentAddBottom,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Select,
  Popconfirm,
  Typography,
  Input,
  Tooltip,
  Toast,
} from '@coze-arch/coze-design';

import { type LogicValue } from '@/shared/components/analytics-logic-expr/logic-expr';

export type View = ListViewsResponse['views'][number];

import { safeJsonParse } from '@/shared/utils/json';
import { useLocale } from '@/i18n';
import styles from './custom-view.module.less';

interface ViewDeleteProps {
  view: View;
  onConfirm: (view: View) => Promise<void>;
  onDelete?: () => void;
}
const ViewDelete = (props: ViewDeleteProps) => {
  const { view, onConfirm, onDelete } = props;
  const [visible, setVisible] = useState(false);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const { t } = useLocale();
  return (
    <Popconfirm
      position="right"
      okButtonColor="red"
      content={t('confirm_delete_view')}
      title={t('deletion_irreversible')}
      onConfirm={async () => {
        setDeleteLoading(true);
        try {
          await onConfirm(view);
          onDelete?.();
        } catch (error) {
          console.error(error);
        } finally {
          setDeleteLoading(false);
          setVisible(false);
        }
      }}
      onCancel={() => setVisible(false)}
      okText={t('button_confirm')}
      cancelText={t('button_cancel')}
      visible={visible}
      trigger="custom"
      stopPropagation
      onVisibleChange={setVisible}
      okButtonProps={{ loading: deleteLoading }}
    >
      <Button
        className="w-[24px] h-[24px] box-border p-1"
        color="secondary"
        size="mini"
        disabled={view.is_system}
        onClick={e => {
          e.stopPropagation();
          e.preventDefault();

          if (!view.is_system) {
            setVisible(true);
          }
        }}
      >
        <IconCozTrashCan className="w-[14px] h-[14px] !coz-fg-secondary" />
      </Button>
    </Popconfirm>
  );
};

interface CustomViewProps {
  onSelectView: (view: null | View) => void;
  viewList: View[];
  activeViewKey: string | null | number;
  onDeleteView: (view: View) => void;
  onUpdateView: (view: View) => void;
  onCreateView: (viewName: string) => void;
  updateViewList?: () => void;
  selectedPlatform: PlatformType;
  onSelectedPlatformChange?: (platform: PlatformType) => void;
  selectedSpanType: SpanListType;
  onSelectedSpanTypeChange?: (spanType: SpanListType) => void;
  customParams?: Record<string, any>;
  filters: LogicValue;
  onFiltersChange?: (filters: LogicValue) => void;
  lastUserRecord: {
    filters: LogicValue;
    selectedPlatform: string;
    selectedSpanType: string;
  };
  onFieldMetasChange?: (
    fieldMetas: Record<string, FieldMeta> | undefined,
  ) => void;
  disabled?: boolean;
}
const CustomView = (props: CustomViewProps) => {
  const {
    onSelectView,
    viewList,
    activeViewKey,
    onDeleteView,
    onUpdateView,
    onCreateView,
    updateViewList,
    onFieldMetasChange,
    onSelectedPlatformChange,
    onSelectedSpanTypeChange,
    lastUserRecord,
    selectedPlatform: selectedPlatformType,
    selectedSpanType: selectedSpanTypeType,
    customParams,
    filters,
    onFiltersChange,
    disabled,
  } = props;

  // 将 viewList 转换为 Select 组件需要的选项格式
  const selectOptions = viewList.map(view => ({
    value: view.id.toString(),
    label: view.view_name,
    view,
  }));

  const handleDeleteView = async (view: View) => {
    try {
      await observabilityTrace.DeleteView({
        view_id: view.id,
        workspace_id: customParams?.spaceID ?? '',
      });
      Toast.success(t('view_delete_success'));
      onDeleteView(view);
    } catch (e) {
      console.error(e);
    }
  };

  const handleUpdateView = async (
    view: Omit<View, 'is_system' | 'filters'> & { filters: LogicValue },
  ) => {
    try {
      await observabilityTrace.UpdateView({
        view_id: view.id,
        view_name: view.view_name,
        filters: JSON.stringify(view.filters),
        span_list_type: view.spanList_type,
        platform_type: view.platform_type,
        workspace_id: customParams?.spaceID ?? '',
      });
      onUpdateView({
        id: view.id,
        view_name: view.view_name,
        filters: JSON.stringify(view.filters),
        spanList_type: view.spanList_type,
        platform_type: view.platform_type,
        workspace_id: customParams?.spaceID ?? '',
        is_system: false,
      });
      Toast.success(t('view_save_success'));
      updateViewList?.();
    } catch (e) {
      console.error(e);
    }
  };

  const { t } = useLocale();
  const handleSaveCurrentEditView = async (
    view: View,
    currentFilter: {
      filters: LogicValue;
      viewMethod: string | number;
      dataSource: string | number;
    },
  ) => {
    await handleUpdateView({
      id: view.id,
      view_name: view.view_name,
      filters: currentFilter.filters,
      spanList_type: currentFilter.viewMethod as SpanListType,
      platform_type: currentFilter.dataSource as PlatformType,
      workspace_id: customParams?.spaceID ?? '',
    });
  };

  const handleSelectView = (view: View) => {
    if (activeViewKey === view.id.toString()) {
      const {
        filters: applyFilters,
        selectedPlatform,
        selectedSpanType,
      } = lastUserRecord;
      onSelectView(null);
      if (
        selectedPlatform !== selectedPlatformType ||
        selectedSpanType !== selectedSpanTypeType
      ) {
        onFieldMetasChange?.(undefined);
      }
      onSelectedPlatformChange?.(
        (selectedPlatform ?? PlatformType.Cozeloop) as unknown as PlatformType,
      );
      onSelectedSpanTypeChange?.(
        (selectedSpanType ?? SpanListType.RootSpan) as unknown as SpanListType,
      );
      onFiltersChange?.(applyFilters ?? { filter_fields: [] });
      return;
    }
    onSelectView(view);

    const viewPlatformType = view.platform_type ?? PlatformType.Cozeloop;
    const viewSpanType = view.spanList_type ?? SpanListType.RootSpan;

    if (
      viewPlatformType !== selectedPlatformType ||
      viewSpanType !== selectedSpanTypeType
    ) {
      onFieldMetasChange?.(undefined);
    }

    onSelectedPlatformChange?.(viewPlatformType);
    onSelectedSpanTypeChange?.(viewSpanType);
    onFiltersChange?.(
      view.filters ? (safeJsonParse(view.filters) as unknown as any) || {} : {},
    );
  };

  interface ViewItemProps {
    view: View;
    onUpdateView: (view: View) => void;
    onDeleteView: (view: View) => void;
  }

  const ViewItem = (viewItemProps: ViewItemProps) => {
    const { view } = viewItemProps;

    const [editing, setEditing] = useState(false);
    const [viewName, setViewName] = useState(view.view_name);
    return (
      <div className="flex items-center justify-between w-full ml-1">
        {editing ? (
          <div
            className="flex items-center gap-x-1"
            onClick={e => {
              e.stopPropagation();
              e.preventDefault();
            }}
          >
            <Input
              value={viewName}
              onChange={name => setViewName(name)}
              size="small"
            />
            <Button
              icon={<IconCozCheckMark className="w-[12px] h-[12px]" />}
              size="mini"
              className="w-[24px] h-[24px] box-border"
              onClick={() => {
                viewItemProps.onUpdateView({
                  ...view,
                  view_name: viewName,
                });
                setEditing(false);
              }}
            />
          </div>
        ) : (
          <>
            <span className="text-[14px] text-[var(--coz-fg-primary)] flex-1 text-ellipsis overflow-hidden whitespace-nowrap">
              {view.view_name}
            </span>

            <div className="flex items-center ml-2 custom-view-button-group">
              {!view.is_system && (
                <Tooltip content={t('edit_name')} theme="dark">
                  <Button
                    className="w-[24px] h-[24px] box-border"
                    onClick={e => {
                      e.stopPropagation();
                      e.preventDefault();
                      setEditing(true);
                    }}
                    color="secondary"
                    size="mini"
                  >
                    <IconCozEdit className="!coz-fg-secondary" />
                  </Button>
                </Tooltip>
              )}
              {!view.is_system && (
                <Tooltip
                  content={t('save_current_filter_to_view')}
                  theme="dark"
                >
                  <Button
                    color="secondary"
                    className="w-[24px] h-[24px] box-border"
                    size="mini"
                    onClick={e => {
                      e.stopPropagation();
                      e.preventDefault();
                      handleSaveCurrentEditView(view, {
                        filters: filters ?? { filter_fields: [] },
                        viewMethod: selectedPlatformType,
                        dataSource: selectedPlatformType,
                      });
                    }}
                  >
                    <IconCozDocumentAddBottom className="!coz-fg-secondary" />
                  </Button>
                </Tooltip>
              )}
              {!view.is_system && (
                <Tooltip content={t('delete')} theme="dark">
                  <span>
                    <ViewDelete
                      view={view}
                      onConfirm={handleDeleteView}
                      onDelete={() => updateViewList?.()}
                    />
                  </span>
                </Tooltip>
              )}
            </div>
          </>
        )}
      </div>
    );
  };

  const handleCreateView = (viewName: string) => {
    try {
      observabilityTrace
        .CreateView({
          workspace_id: customParams?.spaceID ?? '',
          view_name: viewName,
          platform_type: selectedPlatformType as PlatformType,
          span_list_type: selectedSpanTypeType as SpanListType,
          filters: JSON.stringify(filters ?? { filter_fields: [] }),
        })
        .then(() => {
          onCreateView(viewName);
        });
    } catch (e) {
      console.error(e);
    }
  };

  const InnerBottomSlot = () => {
    const [editing, setEditing] = useState(false);
    const [viewName, setViewName] = useState('');
    return (
      <div className="flex items-center justify-between w-full h-[32px]">
        {editing ? (
          <div
            className="flex items-center gap-x-1 ml-[28px]"
            onClick={e => {
              e.stopPropagation();
              e.preventDefault();
            }}
          >
            <Input
              value={viewName}
              onChange={name => setViewName(name)}
              className="w-full h-full"
            />
            <Button
              icon={<IconCozCheckMark className="w-[12px] h-[12px]" />}
              size="mini"
              onClick={() => {
                setEditing(false);
                handleCreateView(viewName);
              }}
              disabled={viewName.trim() === ''}
            />
          </div>
        ) : (
          <>
            <div
              className="flex items-center gap-x-1 text-brand text-[14px] px-2 cursor-pointer"
              onClick={() => {
                setEditing(true);
              }}
            >
              <IconCozPlus />
              <Typography.Text
                className="text-inherit"
                ellipsis={{
                  showTooltip: {
                    opts: {
                      theme: 'dark',
                    },
                  },
                }}
              >
                {t('save_current_filter_as_new_view')}
              </Typography.Text>
            </div>
          </>
        )}
      </div>
    );
  };

  return (
    <div className=" !h-[32px] flex items-center text-sm box-border">
      <Select
        disabled={disabled}
        prefix={
          <IconCozTemplate className="w-[16px] h-[16px] coz-fg-secondary ml-1" />
        }
        className={styles['custom-view-select']}
        placeholder={t('custom_view')}
        value={activeViewKey?.toString() || undefined}
        onChange={value => {
          const selectedView = viewList.find(
            view => view.id.toString() === value,
          );
          if (selectedView) {
            handleSelectView(selectedView);
          }
        }}
        outerBottomSlot={<InnerBottomSlot />}
        style={{ width: '100%', height: '32px' }}
        dropdownStyle={{
          maxHeight: 260,
          minWidth: 260,
          width: 260,
          overflow: 'auto',
        }}
        onClear={() => {
          onSelectView(null);
          const {
            filters: applyFilters,
            selectedPlatform,
            selectedSpanType,
          } = lastUserRecord;
          if (
            selectedPlatform !== selectedPlatformType ||
            selectedSpanType !== selectedSpanTypeType
          ) {
            onFieldMetasChange?.(undefined);
          }
          onSelectedPlatformChange?.(
            (selectedPlatform ??
              PlatformType.Cozeloop) as unknown as PlatformType,
          );
          onSelectedSpanTypeChange?.(
            (selectedSpanType ??
              SpanListType.RootSpan) as unknown as SpanListType,
          );
          onFiltersChange?.(applyFilters ?? { filter_fields: [] });
        }}
      >
        {selectOptions.map(option => (
          <Select.Option value={option.value} key={option.view.view_name}>
            <ViewItem
              view={option.view}
              key={option.view.view_name}
              onUpdateView={v => {
                handleUpdateView({
                  ...v,
                  filters: JSON.parse(v.filters),
                });
              }}
              onDeleteView={handleDeleteView}
            />
          </Select.Option>
        ))}
      </Select>
    </div>
  );
};
export { CustomView };
