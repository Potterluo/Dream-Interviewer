// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 此处后端没有提供注释 GET / */
export async function check(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.checkParams,
  options?: { [key: string]: any },
) {
  return request<string>('/', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 POST / */
export async function receiveMessage(options?: { [key: string]: any }) {
  return request<any>('/', {
    method: 'POST',
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 GET /setMenu */
export async function setMenu(options?: { [key: string]: any }) {
  return request<string>('/setMenu', {
    method: 'GET',
    ...(options || {}),
  });
}
