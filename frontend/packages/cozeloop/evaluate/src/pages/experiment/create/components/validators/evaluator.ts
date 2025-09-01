export const evaluatorValidators = {
  evaluatorProList: [
    { required: true, message: '请添加评估器' },
    { type: 'array', min: 1, message: '至少添加一个评估器' },
    { type: 'array', max: 5, message: '最多添加5个评估器' },
  ],
};
