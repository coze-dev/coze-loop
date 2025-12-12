import { useRef, useState } from 'react';

import { InfoTooltip, InputSlider, useI18n } from '@cozeloop/components';
import { type ContentPart } from '@cozeloop/api-schema/prompt';
import { IconCozSetting } from '@coze-arch/coze-design/icons';
import {
  Form,
  type FormApi,
  IconButton,
  Modal,
  Tooltip,
  Typography,
  withField,
} from '@coze-arch/coze-design';

const InputSliderFormItem = withField(InputSlider);

interface VideoConfigProps {
  value?: ContentPart;
  onChange?: (value: ContentPart) => void;
}

export function VideoConfig({ value, onChange }: VideoConfigProps) {
  const formApi = useRef<FormApi<ContentPart>>();
  const [visible, setVisible] = useState(false);
  const I18n = useI18n();

  return (
    <>
      <Tooltip
        content={
          <>
            {I18n.t('fornax_prompt_manual_config_video_sample_params')}
            <a
              href="https://bytedance.larkoffice.com/wiki/XyDewau0ViBRvJkEKWIcsNcZnEd#share-XUZ7dOemgovOFOxGS8ocCnPNnqf"
              target="_blank"
              className="ml-0.5 !coz-fg-color-blue no-underline hover:!coz-fg-hglt"
            >
              {I18n.t('fornax_prompt_documentation')}
            </a>
          </>
        }
      >
        <IconButton
          icon={<IconCozSetting />}
          size="mini"
          color="secondary"
          onClick={() => {
            setVisible(true);
          }}
        />
      </Tooltip>
      <Modal
        title={
          <span className="flex items-center gap-1">
            {I18n.t('fornax_prompt_video_sampling_config')}
            <InfoTooltip
              content={
                <>
                  {I18n.t(
                    'fornax_prompt_video_manual_uniform_sampling_support',
                  )}
                  <a
                    href="https://bytedance.larkoffice.com/wiki/XyDewau0ViBRvJkEKWIcsNcZnEd#share-XUZ7dOemgovOFOxGS8ocCnPNnqf"
                    target="_blank"
                    className="ml-0.5 !coz-fg-color-blue no-underline hover:!coz-fg-hglt"
                  >
                    {I18n.t('prompt_view_documentation')}
                  </a>
                </>
              }
            />
          </span>
        }
        visible={visible}
        onCancel={() => setVisible(false)}
        onOk={async () => {
          const values = await formApi.current
            ?.validate()
            ?.catch(err => console.error(err));
          if (values) {
            setVisible(false);
            onChange?.(values);
          }
        }}
        okText={I18n.t('global_btn_confirm')}
        cancelText={I18n.t('cancel')}
      >
        <Form<ContentPart>
          getFormApi={api => (formApi.current = api)}
          initValues={{ media_config: value?.media_config }}
        >
          <Form.Label required>
            {I18n.t('fornax_prompt_frame_extraction_config')}
          </Form.Label>
          <InputSliderFormItem
            fieldClassName="!pt-0.5 !pb-0"
            label={{
              text: (
                <Typography.Text
                  className="inline font-medium"
                  type="secondary"
                >
                  FPS
                </Typography.Text>
              ),

              required: true,
              extra: (
                <InfoTooltip
                  content={I18n.t(
                    'fornax_prompt_fps_influence_on_video_understanding_and_token_usage',
                  )}
                />
              ),
            }}
            field="media_config.fps"
            labelPosition="left"
            rules={[
              {
                required: true,
                message: I18n.t(
                  'fornax_prompt_please_enter_frame_extraction_config',
                ),
              },
            ]}
            min={0.2}
            max={5}
            step={0.1}
          />
        </Form>
      </Modal>
    </>
  );
}
