import {PageContainer,  ProForm} from '@ant-design/pro-components';
import {Card, message,} from 'antd';
import React from 'react';
import { Col, Row } from 'antd';
import {updateUser} from "@/services/backend/userController";

import {ProFormText} from "@ant-design/pro-form";
import {useModel} from "@@/exports";



/**
 * 更新节点
 *
 * @param fields
 */
const handleUpdate = async (fields: API.UserUpdateRequest) => {
  const hide = message.loading('正在更新');
  try {
    await updateUser(fields);
    hide();
    message.success('更新成功');
    return true;
  } catch (error: any) {
    hide();
    message.error('更新失败，' + error.message);
    return false;
  }
};

const UpdateUser: React.FC = () => {
  const { initialState} = useModel('@@initialState');
  const { currentUser } = initialState || {};

  return (
    <PageContainer>
      <Row>
        <Col span={9}></Col>
        <Col span={6}>
          <Card>
            <h2>用户信息修改</h2>

            <ProForm
              initialValues={currentUser}
              layout={'horizontal'}
            >
              <ProFormText
                name="userAccount"
                width="md"
                label="账号"

              />
              <ProFormText
                width="md"
                name="userName"
                label="昵称"
                placeholder="请输入名称"
              />
              <ProFormText
                width="md"
                name="userAvatar"
                label="头像"
                placeholder="请输入名称"
              />
            </ProForm>
          </Card>
        </Col>
        <Col span={9}></Col>
      </Row>

    </PageContainer>
  );
};

export default UpdateUser;
