import {GithubOutlined, RobotOutlined} from '@ant-design/icons';
import { DefaultFooter } from '@ant-design/pro-components';
import '@umijs/max';
import React from 'react';

const Footer: React.FC = () => {
  const defaultMessage = 'keriko';
  const currentYear = new Date().getFullYear();
  return (
    <DefaultFooter
      style={{
        background: 'none',
      }}
      copyright={`${currentYear} ${defaultMessage}`}
      links={[
        {
          key: 'codeNav',
          title: (
            <>
              <RobotOutlined /> 个人博客
            </>
          ),
          href: 'http://keriko.fun/',
          blankTarget: true,
        },
        {
          key: 'github',
          title: (
            <>
              <GithubOutlined />项目源码
            </>
          ),
          href: 'https://github.com/Potterluo/Dream-Interviewer',
          blankTarget: true,
        },
      ]}
    />
  );
};
export default Footer;
