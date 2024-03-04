// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** addMessageList POST /api/message/add */
export async function addMessageListUsingPost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.addMessageListUsingPOSTParams,
  options?: { [key: string]: any },
) {
  return request<number>('/api/message/add', {
    method: 'POST',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** deleteMessageList GET /api/message/delete */
export async function deleteMessageListUsingGet(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteMessageListUsingGETParams,
  options?: { [key: string]: any },
) {
  return request<boolean>('/api/message/delete', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** listMessageList GET /api/message/list */
export async function listMessageListUsingGet(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listMessageListUsingGETParams,
  options?: { [key: string]: any },
) {
  return request<API.MessageList[]>('/api/message/list', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** mainMessage GET /api/message/main */
export async function mainMessageUsingGet(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.mainMessageUsingGETParams,
  options?: { [key: string]: any },
) {
  return request<string>('/api/message/main', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}
