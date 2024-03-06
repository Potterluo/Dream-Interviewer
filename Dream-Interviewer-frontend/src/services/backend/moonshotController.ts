// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 此处后端没有提供注释 GET /moonshot/test */
export async function chat1(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.chat1Params,
  options?: { [key: string]: any },
) {
  return request<string>('/moonshot/test', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 GET /moonshot/test2 */
export async function chat2(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.chat2Params,
  options?: { [key: string]: any },
) {
  return request<string>('/moonshot/test2', {
    method: 'GET',
    params: {
      ...params,
      sendMessage: undefined,
      ...params['sendMessage'],
    },
    ...(options || {}),
  });
}
