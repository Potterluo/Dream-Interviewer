// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 此处后端没有提供注释 POST /context/add */
export async function addContext(body: API.ContextRequest, options?: { [key: string]: any }) {
  return request<API.ContextRequest>('/context/add', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 POST /context/chat */
export async function chat(body: API.ContextRequest, options?: { [key: string]: any }) {
  return request<API.ContextRequest>('/context/chat', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 POST /context/list */
export async function listContext(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listContextParams,
  options?: { [key: string]: any },
) {
  return request<API.Context[]>('/context/list', {
    method: 'POST',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}
