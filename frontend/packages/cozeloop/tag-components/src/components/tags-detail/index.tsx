// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useParams, useBlocker } from 'react-router-dom';
import { useEffect, useState, useRef } from 'react';

import { isEqual } from 'lodash-es';
import { I18n } from '@cozeloop/i18n-adapter';
import { useBreadcrumb } from '@cozeloop/hooks';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import { useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { Layout, Modal, Spin, Toast } from '@coze-arch/coze-design';

import { formatTagDetailToFormValues } from '@/utils';
import { useUpdateTag } from '@/hooks/use-update-tag';
import { useGetTagSpec } from '@/hooks/use-get-tag-spec';
import { useFetchTagDetail } from '@/hooks/use-fetch-tag-detail';
import { type FormValues } from '@/components/tags-form';
import { EditHistoryList } from '@/components/edit-history-list';

import { TagDetailHeader } from './header';
import { TagDetailContent, type TagDetailContentRef } from './content';

interface TagsDetailProps {
  /**
   * 标签列表路由路径，用于跳转和拼接 标签列表 路由路径
   */
  tagListPagePath?: string;
  /**
   * 标签列表参数，用于跳转和拼接 标签详情 / 创建标签 路由路径的查询参数
   */
  tagListPageQuery?: string;
}
export const TagsDetail = ({
  tagListPagePath,
  tagListPageQuery,
}: TagsDetailProps) => {
  const navigate = useNavigateModule();
  // 展示编辑历史列表
  const [editHistoryVisible, setEditHistoryVisible] = useState(false);
  // 是否修改过
  const [changed, setChanged] = useState(false);
  // 是否离开页面
  const [blockLeave, setBlockLeave] = useState(false);

  const guard = useGuard({
    point: GuardPoint['data.tag.edit'],
  });
  const { tagId } = useParams<{ tagId: string }>();

  // 请求标签详情
  const { run, data, loading } = useFetchTagDetail();
  // 请求标签规格
  const { data: tagSpec, loading: tagSpecLoading } = useGetTagSpec();
  // 更新标签
  const { runAsync: updateTag } = useUpdateTag();

  const contentRef = useRef<TagDetailContentRef>(null);

  // 页面离开拦截
  const blocker = useBlocker(({ currentLocation, nextLocation }) => {
    if (blockLeave && currentLocation.pathname !== nextLocation.pathname) {
      return true;
    }
    return false;
  });

  useEffect(() => {
    if (blocker.state === 'blocked') {
      Modal.warning({
        title: I18n.t('exit_edit'),
        content: I18n.t('unsaved_changes_exit_warning'),
        cancelText: I18n.t('cancel'),
        onCancel: blocker.reset,
        okText: I18n.t('exit'),
        onOk: blocker.proceed,
      });
    }
  }, [blocker.state]);

  useEffect(() => {
    run({ tagKeyID: tagId ?? '' });
  }, [tagId]);

  const tagDetail = data?.tags?.[0];

  useBreadcrumb({
    text: tagDetail?.tag_key_name ?? '',
  });

  const handleValueChange = (values: FormValues) => {
    const valueChanged = !isEqual(
      values,
      formatTagDetailToFormValues(tagDetail || {}),
    );
    if (valueChanged) {
      setChanged(true);
      setBlockLeave(true);
    } else {
      setChanged(false);
      setBlockLeave(false);
    }
  };

  const handleSubmit = (values: FormValues) => {
    Modal.confirm({
      title: I18n.t('confirm_save'),
      content: I18n.t('tag_edit_sync_warning'),
      onOk: () => {
        setChanged(false);
        setBlockLeave(false);
        updateTag(values)
          .then(() => {
            Toast.success(I18n.t('save_success'));
            setTimeout(() => {
              navigate(
                `${tagListPagePath}${tagListPageQuery ? `?${tagListPageQuery}` : ''}`,
              );
            }, 300);
          })
          .catch(err => console.error(err));
      },
      okText: I18n.t('save'),
      cancelText: I18n.t('cancel'),
      width: 420,
      autoLoading: true,
      cancelButtonProps: {
        color: 'primary',
      },
      okButtonProps: {
        color: 'brand',
      },
    });
  };

  const handleHeaderSubmit = () => {
    contentRef.current?.submit();
  };

  if (loading || tagSpecLoading) {
    return (
      <div className="h-full w-full flex items-center justify-center">
        <Spin />
      </div>
    );
  }

  return (
    <Layout.Content className="w-full h-full overflow-hidden flex flex-col !px-0">
      <div className="flex h-full flex-col">
        <TagDetailHeader
          tagDetail={tagDetail}
          onShowEditHistory={() => setEditHistoryVisible(true)}
          onSubmit={handleHeaderSubmit}
          changed={changed}
          guardType={guard.data.type}
          tagListPagePath={tagListPagePath}
          tagListPageQuery={tagListPageQuery}
        />

        <div className="flex flex-1 h-full overflow-hidden">
          <TagDetailContent
            ref={contentRef}
            tagDetail={tagDetail}
            tagSpec={tagSpec}
            onValueChange={handleValueChange}
            onSubmit={handleSubmit}
          />

          {editHistoryVisible ? (
            <div>
              <EditHistoryList onClose={() => setEditHistoryVisible(false)} />
            </div>
          ) : null}
        </div>
      </div>
    </Layout.Content>
  );
};
