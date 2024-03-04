// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** chat GET /api/moonshot/test */
export async function chatUsingGet(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.chatUsingGETParams,
  options?: { [key: string]: any },
) {
  return request<string>('/api/moonshot/test', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** chat2 GET /api/moonshot/test2 */
export async function chat2UsingGet(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.chat2UsingGETParams,
  options?: { [key: string]: any },
) {
  return request<string>('/api/moonshot/test2', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}
