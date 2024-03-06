// @ts-ignore
/* eslint-disable */
import { request } from '@umijs/max';

/** 此处后端没有提供注释 POST /post_thumb/ */
export async function doThumb(body: API.PostThumbAddRequest, options?: { [key: string]: any }) {
  return request<API.BaseResponseInteger>('/post_thumb/', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}
