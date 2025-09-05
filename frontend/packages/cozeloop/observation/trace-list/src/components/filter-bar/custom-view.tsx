// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */
import { Fragment, useState } from 'react';

import classNames from 'classnames';
import { useDebounceFn } from 'ahooks';
import { safeJsonParse } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import { TooltipWhenDisabled } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  PlatformType,
  type SpanListType,
  type ListViewsResponse,
} from '@cozeloop/api-schema/observation';
import { observabilityTrace } from '@cozeloop/api-schema';
import {
  IconCozPlus,
  IconCozSetting,
  IconCozEdit,
  IconCozEye,
  IconCozEyeClose,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  Dropdown,
  Popconfirm,
  Tooltip,
} from '@coze-arch/coze-design';

import { useTraceStore } from '@/stores/trace';
import { SpanType } from '@/consts';
import { FilterSelectUI } from '@/components/filter-select-ui';

const MAX_VIEW_COUNT = 5;

import {
  type LogicValue,
  type CustomRightRenderMap,
} from '../logic-expr/logic-expr';
import { validateViewName } from '../../utils/name-validate';

export type View = ListViewsResponse['views'][number];

import styles from './custom-view.module.less';

const VIEW_TOOLTIPS = {
  '-1': 'Status Error',
  '-2': 'Latency > 10s',
};

interface ViewDeleteProps {
  view: View;
  onConfirm: (view: View) => Promise<void>;
}
const ViewDelete = (props: ViewDeleteProps) => {
  const { view, onConfirm } = props;
  const [visible, setVisible] = useState(false);
  const [deleteLoading, setDeleteLoading] = useState(false);
  return (
    <Popconfirm
      zIndex={1001}
      okButtonColor="red"
      content={I18n.t('confirm_delete_view')}
      title={I18n.t('deletion_irreversible')}
      onConfirm={async () => {
        setDeleteLoading(true);
        try {
          await onConfirm(view);
        } finally {
          setDeleteLoading(false);
          setVisible(false);
        }
      }}
      onCancel={() => setVisible(false)}
      okText={I18n.t('confirm')}
      cancelText={I18n.t('cancel')}
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
        <IconCozTrashCan className="w-[14px] h-[14px]" />
      </Button>
    </Popconfirm>
  );
};

interface CustomViewProps {
  onSelectView: (view: null | View) => void;
  viewList: View[];
  visibleViewIds: (string | number)[];
  onTriggerViewVisible: (view: View) => void;
  viewNames: string[];
  activeViewKey: string | null | number;
  onDelteView: (view: View) => void;
  onUpdateView: (view: View) => void;
  customRightRenderMap: CustomRightRenderMap;
  customLeftRenderMap: CustomRightRenderMap;
  platformEnumOptionList: { label: string; value: string | number }[];
  spanListTypeEnumOptionList: { label: string; value: string | number }[];
}
const CustomView = (props: CustomViewProps) => {
  const {
    onSelectView,
    viewList,
    onTriggerViewVisible,
    visibleViewIds,
    viewNames,
    activeViewKey,
    onDelteView,
    onUpdateView,
    customRightRenderMap,
    customLeftRenderMap,
    platformEnumOptionList,
    spanListTypeEnumOptionList,
  } = props;
  const {
    fieldMetas,
    setFieldMetas,
    setSelectedPlatform,
    setSelectedSpanType,
    setFilters,
    setApplyFilters,
    setFilterPopupVisible,
    filterPopupVisible,
    lastUserRecord,
    selectedPlatform: selectedPlatformType,
    selectedSpanType: selectedSpanTypeType,
  } = useTraceStore();

  const { spaceID } = useSpace();
  const [templateShowView, setTemplateShowView] = useState<View | null>(null);
  const { run: openFilterPopup } = useDebounceFn(
    () => {
      if (filterPopupVisible) {
        return;
      }
      setFilterPopupVisible(true);
    },
    {
      wait: 200,
    },
  );

  const handleDeleteView = async (view: View) => {
    try {
      await observabilityTrace.DeleteView({
        view_id: view.id,
        workspace_id: spaceID,
      });
      onDelteView(view);
    } catch (e) {
      console.log(e);
    }
  };

  const handleUpdateView = async (
    newView: Omit<View, 'is_system' | 'filters'> & { filters: LogicValue },
  ) => {
    try {
      await observabilityTrace.UpdateView({
        view_id: newView.id,
        view_name: newView.view_name,
        filters: JSON.stringify(newView.filters),
        span_list_type: newView.spanList_type,
        platform_type: newView.platform_type,
        workspace_id: spaceID,
      });
      onUpdateView({
        id: newView.id,
        view_name: newView.view_name,
        filters: JSON.stringify(newView.filters),
        spanList_type: newView.spanList_type,
        platform_type: newView.platform_type,
        workspace_id: spaceID,
        is_system: false,
      });
    } catch (e) {
      console.log(e);
    }
  };

  const [editViewVisible, setEditViewVisible] = useState(false);
  const [editViewId, setEditViewId] = useState('');
  const [viewListVisible, setViewListVisible] = useState(false);
  const handleSaveCurrentEditView = (
    view: View,
    currentFilter: {
      filters: LogicValue;
      viewMethod: string | number;
      dataSource: string | number;
    },
  ) => {
    const { filters } = currentFilter;
    const newView = {
      id: view.id,
      view_name: view.view_name,
      filters,
      spanList_type: currentFilter.viewMethod as SpanListType,
      platform_type: currentFilter.dataSource as PlatformType,
      workspace_id: spaceID,
    };
    handleUpdateView({
      ...newView,
    });
  };

  const handleSelectView = (view: View) => {
    if (activeViewKey === view.id.toString()) {
      const { filters, selectedPlatform, selectedSpanType } = lastUserRecord;
      onSelectView(null);
      if (
        selectedPlatform !== selectedPlatformType ||
        selectedSpanType !== selectedSpanTypeType
      ) {
        setFieldMetas(undefined);
      }
      setSelectedPlatform(selectedPlatform ?? PlatformType.Cozeloop);
      setSelectedSpanType(selectedSpanType ?? SpanType.RootSpan);
      setApplyFilters(filters ?? { filter_fields: [] });
      setFilters(filters ?? { filter_fields: [] });
      return;
    }
    onSelectView(view);

    const viewPlatformType = view.platform_type ?? PlatformType.Cozeloop;
    const viewSpanType = view.spanList_type ?? SpanType.RootSpan;

    if (
      viewPlatformType !== selectedPlatformType ||
      viewSpanType !== selectedSpanTypeType
    ) {
      setFieldMetas(undefined);
    }

    setSelectedPlatform(viewPlatformType);
    setSelectedSpanType(viewSpanType);
    setApplyFilters(view.filters ? safeJsonParse(view.filters) || {} : {});
    setFilters(view.filters ? safeJsonParse(view.filters) || {} : {});
  };

  return (
    <div className="rounded-[6px] !h-[32px] border border-solid border-[var(--coz-stroke-plus)] flex items-center text-sm box-border overflow-visible">
      <Dropdown
        trigger="custom"
        position="bottomLeft"
        zIndex={1000}
        visible={viewListVisible}
        onClickOutSide={() => {
          if (editViewVisible) {
            return;
          }
          setViewListVisible(false);
        }}
        onVisibleChange={visible => {
          if (!visible) {
            setTemplateShowView(null);
          }
        }}
        render={
          <Dropdown.Menu className="overflow-x-hidden overflow-y-auto box-border max-h-[300px]">
            {viewList.map(view => (
              <Dropdown.Item
                className={styles['dropdown-item']}
                key={view.id}
                onClick={() => {
                  handleSelectView(view);
                  if (!visibleViewIds.includes(view.id)) {
                    setTemplateShowView(view);
                  }
                }}
              >
                <div className="flex items-center flex-nowrap justify-between w-full max-w-[400px] overflow-auto h-full">
                  <div className="text-[13px] text-[var(--coz-fg-primary)] flex-1 max-w-[180px] text-ellipsis overflow-hidden whitespace-nowrap">
                    {view.view_name}
                  </div>
                  <div className="flex items-center gap-x-1">
                    <TooltipWhenDisabled
                      theme="dark"
                      disabled={
                        visibleViewIds.length >= MAX_VIEW_COUNT &&
                        !visibleViewIds.includes(view.id)
                      }
                      content={I18n.t('max_display_view_num', { num: 5 })}
                    >
                      <Button
                        className={classNames(
                          'w-[24px] h-[24px] box-border p-1',
                        )}
                        style={
                          (visibleViewIds.length >= MAX_VIEW_COUNT &&
                            !visibleViewIds.includes(view.id)) ||
                          templateShowView?.id === view.id
                            ? {
                                color:
                                  'rgba(var(--coze-fg-1), var(--coze-fg-1-alpha))',
                                backgroundColor: '#0000',
                                cursor: 'not-allowed',
                              }
                            : {}
                        }
                        color="secondary"
                        size="mini"
                        onClick={e => {
                          e.stopPropagation();
                          if (
                            (visibleViewIds.length >= MAX_VIEW_COUNT &&
                              !visibleViewIds.includes(view.id)) ||
                            templateShowView?.id === view.id
                          ) {
                            return;
                          }
                          onTriggerViewVisible(view);
                        }}
                      >
                        {!visibleViewIds.includes(view.id) &&
                        templateShowView?.id !== view.id ? (
                          <IconCozEyeClose className="w-[14px] h-[14px]" />
                        ) : (
                          <IconCozEye className="w-[14px] h-[14px]" />
                        )}
                      </Button>
                    </TooltipWhenDisabled>
                    <FilterSelectUI
                      customRightRenderMap={customRightRenderMap}
                      customLeftRenderMap={customLeftRenderMap}
                      spanTabOptionList={spanListTypeEnumOptionList}
                      fieldMetas={fieldMetas}
                      filters={
                        view.filters ? safeJsonParse(view.filters) || {} : {}
                      }
                      dataSource={(view.platform_type ?? '') as string}
                      viewMethod={(view.spanList_type ?? '') as string}
                      onVisibleChange={visible => {
                        setEditViewVisible(visible);
                        if (!visible) {
                          setEditViewId('');
                        }
                      }}
                      platformEnumOptionList={platformEnumOptionList}
                      visible={editViewId === view.id.toString()}
                      customFooter={({ onCancel, onSave, currentFilter }) => (
                        <div className="flex justify-end gap-x-2">
                          <Button
                            type="primary"
                            color="secondary"
                            onClick={onCancel}
                          >
                            {I18n.t('confirm')}
                          </Button>
                          <Button
                            type="primary"
                            color="brand"
                            onClick={async () => {
                              await handleSaveCurrentEditView(
                                view,
                                currentFilter,
                              );
                              onSave?.();
                            }}
                          >
                            {I18n.t('save')}
                          </Button>
                        </div>
                      )}
                      onViewNameValidate={name =>
                        validateViewName(name, viewNames)
                      }
                      triggerRender={
                        <TooltipWhenDisabled
                          disabled={view.is_system}
                          content={I18n.t('default_view_not_editable')}
                          theme="dark"
                        >
                          <div>
                            <Button
                              className="w-[24px] h-[24px] box-border p-1"
                              color="secondary"
                              size="mini"
                              disabled={view.is_system}
                              onClick={e => {
                                console.log('click');
                                e.stopPropagation();
                                e.preventDefault();
                                if (view.is_system) {
                                  return;
                                }
                                setEditViewId(view.id.toString());
                              }}
                            >
                              <IconCozEdit className="w-[14px] h-[14px]" />
                            </Button>
                          </div>
                        </TooltipWhenDisabled>
                      }
                    />
                    <TooltipWhenDisabled
                      disabled={view.is_system}
                      theme="dark"
                      content={I18n.t('default_view_cannot_be_deleted')}
                    >
                      <div>
                        <ViewDelete view={view} onConfirm={handleDeleteView} />
                      </div>
                    </TooltipWhenDisabled>
                  </div>
                </div>
              </Dropdown.Item>
            ))}
          </Dropdown.Menu>
        }
      >
        <Button
          type="primary"
          className="flex !rounded-none !h-full !px-[8px] !py-[4px]"
          color="secondary"
          onClick={() => setViewListVisible(true)}
        >
          <span className="text-[var(--coz-fg-secondary)] font-normal leading-5">
            {I18n.t('custom_view')}
          </span>
          <IconCozSetting className="ml-2" />
        </Button>
      </Dropdown>

      {viewList
        .filter(view => visibleViewIds.includes(view.id))
        .slice(0, MAX_VIEW_COUNT)
        .concat(templateShowView ? [templateShowView] : [])
        .map(view => (
          <Fragment key={view.id}>
            <Divider
              className="!h-[30px] !m-0 !border-[var(--coz-stroke-plus)]"
              layout="vertical"
            />
            {VIEW_TOOLTIPS[view.id.toString() as keyof typeof VIEW_TOOLTIPS] ? (
              <Tooltip
                theme="dark"
                content={
                  VIEW_TOOLTIPS[
                    view.id.toString() as keyof typeof VIEW_TOOLTIPS
                  ]
                }
              >
                <div
                  className={classNames(styles['button-wrapper'], {
                    [styles.active]: view.id.toString() === activeViewKey,
                  })}
                >
                  <Button
                    type="primary"
                    color="secondary"
                    className="flex !rounded-none !h-full"
                    onClick={() => handleSelectView(view)}
                  >
                    <span className="text-sm text-[var(--coz-fg-primary)] max-w-[139px] text-ellipsis overflow-hidden">
                      {view.view_name}
                    </span>
                  </Button>
                </div>
              </Tooltip>
            ) : (
              <div
                className={classNames(styles['button-wrapper'], {
                  [styles.active]: view.id.toString() === activeViewKey,
                })}
              >
                <Button
                  type="primary"
                  color="secondary"
                  className="flex !rounded-none !h-full"
                  onClick={() => handleSelectView(view)}
                >
                  <span className="text-sm text-[var(--coz-fg-primary)] max-w-[139px] text-ellipsis overflow-hidden">
                    {view.view_name}
                  </span>
                </Button>
              </div>
            )}
          </Fragment>
        ))}
      <Divider
        className="!h-[30px] !m-0 !border-[var(--coz-stroke-plus)]"
        layout="vertical"
      />
      <Button
        type="primary"
        color="secondary"
        className="flex !rounded-none !text-[var(--coz-fg-primary)] !h-full !w-[32px]"
        onClickCapture={e => {
          e.stopPropagation();
          e.preventDefault();
          openFilterPopup();
        }}
      >
        <div className="flex items-center justify-center w-full h-full text-sm">
          <IconCozPlus className="text-[var(--coz-fg-primary)]" />
        </div>
      </Button>
    </div>
  );
};

export { CustomView };
