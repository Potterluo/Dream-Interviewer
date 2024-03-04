// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** addContext POST /api/context/add */
export async function addContextUsingPost(
  body: API.ContextRequest,
  options?: { [key: string]: any },
) {
  return request<API.ContextRequest>('/api/context/add', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** chat POST /api/context/chat */
export async function chatUsingPost(body: API.ContextRequest, options?: { [key: string]: any }) {
  return request<API.ContextRequest>('/api/context/chat', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** listContext POST /api/context/list */
export async function listContextUsingPost(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listContextUsingPOSTParams,
  options?: { [key: string]: any },
) {
  return request<API.Context[]>('/api/context/list', {
    method: 'POST',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}
