// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 此处后端没有提供注释 POST /message/add */
export async function addMessageList(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.addMessageListParams,
  options?: { [key: string]: any },
) {
  return request<number>('/message/add', {
    method: 'POST',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 GET /message/delete */
export async function deleteMessageList(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.deleteMessageListParams,
  options?: { [key: string]: any },
) {
  return request<boolean>('/message/delete', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 GET /message/list */
export async function listMessageList(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.listMessageListParams,
  options?: { [key: string]: any },
) {
  return request<API.MessageList[]>('/message/list', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 GET /message/main */
export async function mainMessage(
  // 叠加生成的Param类型 (非body参数swagger默认没有生成对象)
  params: API.mainMessageParams,
  options?: { [key: string]: any },
) {
  return request<string>('/message/main', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}
