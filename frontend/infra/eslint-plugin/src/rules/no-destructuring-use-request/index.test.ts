import { RuleTester } from 'eslint';
import { noDestructuringUseRequestRule } from './index';

const ruleTester = new RuleTester({});

ruleTester.run('no-destructuring-use-request', noDestructuringUseRequestRule, {
  valid: [
    // 正确的使用方式 - 不解构
    {
      code: 'const request = useRequest();',
    },
    {
      code: 'const myRequest = useRequest(config);',
    },
    {
      code: 'const apiRequest = useRequest({ url: "/api" });',
    },
    // 其他 hook 的解构是允许的
    {
      code: 'const { data, loading } = useState();',
    },
    {
      code: 'const [count, setCount] = useState(0);',
    },
    // 非 useRequest 的函数调用解构
    {
      code: 'const { result } = someOtherFunction();',
    },
    // 变量声明但不是函数调用
    {
      code: 'const { data } = props;',
    },
  ],
  invalid: [
    // 对象解构 useRequest
    {
      code: 'const { data, loading, error } = useRequest();',
      errors: [
        {
          messageId: 'noDestructuring',
        },
      ],
    },
    {
      code: 'const { run, data } = useRequest(config);',
      errors: [
        {
          messageId: 'noDestructuring',
        },
      ],
    },
    // 带参数的 useRequest 解构
    {
      code: 'const { data, loading, error, run } = useRequest({ url: "/api/users" });',
      errors: [
        {
          messageId: 'noDestructuring',
        },
      ],
    },
  ],
});
