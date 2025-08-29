import { useUserStore } from '@cozeloop/account';

export function useUserInfo() {
  const userInfo = useUserStore(s => s.userInfo);

  return {
    app_id: 1,
    user_id_str: userInfo?.user_id,
    email: userInfo?.email,
    screen_name: userInfo?.nick_name,
    name: userInfo?.name,
    avatar_url: userInfo?.avatar_url || '',
  };
}
