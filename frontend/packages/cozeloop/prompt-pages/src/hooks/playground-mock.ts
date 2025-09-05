import { Role, ToolType, VariableType } from '@cozeloop/api-schema/prompt';

import { type PromptState } from '@/store/use-prompt-store';
import { type PromptMockDataState } from '@/store/use-mockdata-store';

export const mockMockSet: PromptMockDataState = {
  historicMessage: [],
};

export const mockInfo: PromptState = {
  modelConfig: {},
  tools: [
    {
      type: ToolType.Function,
      function: {
        name: 'get_weather',
        description: 'Determine weather in my location',
        parameters:
          '{"type":"object","properties":{"location":{"type":"string","description":"The city and state e.g. San Francisco, CA"},"unit":{"type":"string","enum":["c","f"]}},"required":["location"]}',
      },
    },
  ],
  variables: [
    {
      key: 'departure',
      type: VariableType.String,
      desc: '',
    },
    {
      desc: '',
      type: VariableType.String,
      key: 'destination',
    },
    {
      desc: '',
      type: VariableType.String,
      key: 'people_num',
    },
    {
      desc: '',
      type: VariableType.String,
      key: 'days_num',
    },
    {
      type: VariableType.String,
      key: 'travel_theme',

      desc: '',
    },
  ],
  promptInfo: {},
  messageList: [
    {
      key: '1',
      content:
        '# 角色\n你是一个专业的旅游规划助手，能够根据用户的具体需求和偏好，迅速且精准地为用户生成全面、详细且个性化的旅游规划文档。\n\n## 技能：制定旅游规划方案\n为用户量身制定合理且舒适的行程安排和贴心的旅行指引。对于不同主题，需要能够体现对应主题的特色、需求或注意事项等。如亲子游，需要体现带小孩旅行途中要注意的内容，用户的预算和偏好等。 \n回复使用以下格式（内容可以合理使用 emoji 表情，让内容更生动）：\n\n## 输出格式\n#### 基本信息\n- 🛫 出发地：{{departure}}  <如未提供，则不展示此信息>\n- 🎯 目的地：{{destination}}\n- 🫂 人数：{{people_num}}人\n- 📅 天数：{{days_num}}天\n- 🎨 主题：{{travel_theme}} \n##### <目的地>简介\n<介目的地的基本信息，约100字>\n<描述天气状况、穿衣指南，约100字>\n<描述当地特色饮食、风俗习惯等，约100字>\n#### Checklist\n- 手机、充电器\n<需要携带的物品或准备事项，按需求生成>\n#### 行程安排\n<根据用户期望天数（{{days_num}}天）安排每日行程>\n##### 第一天、地点1 - 地点2 - ...\n###### 行程1：地点1\n<地点的景点简介，约100字>\n<地点的交通方式，提供合理的交通方式及使用时间信息>\n<地点的游玩方式，提供推荐游玩时长、游玩方式、注意事项、预定信息等，约100字>\n<如果 {{days_num}}超过1天，则继续按照第一天格式生成>\n#### 注意事项\n<根据以上日程安排信息，提供一些目的地旅行的注意事项>\n\n\n## 限制:\n- 所输出的内容必须按照给定的格式进行组织，不能偏离框架要求。',
      role: Role.System,
    },
    {
      key: '2',
      content:
        '## 用户需求\n- 出发地：{{departure}}  \n- 目的地：{{destination}}\n- 人数：{{people_num}}\n- 天数：{{days_num}}\n- 主题：{{travel_theme}} ',

      role: Role.User,
    },
  ],
};
