import React, { useCallback } from 'react';
import '@chatui/core/es/styles/index.less';
// 引入组件
import Chat, { Bubble, useMessages } from '@chatui/core';
// 引入样式
import './chatui-theme.css';
import {Avatar, Spin} from 'antd';
import {useModel} from "@@/plugin-model/useModel";
import styles from "@/components/RightContent/index.less";



// 定义消息类型
interface Message {
  type: 'text';
  content: { text: string };
  position?: 'left' | 'right';
}
const divStyle: React.CSSProperties = {
  position: 'absolute',
  width: '80%',
  height: '100%', // 设置高度为100%以填充父元素的高度
  left: '50%', // 设置左边距为50%
  transform: 'translateX(-50%)', // 使用transform将元素水平居中

};
const initialMessages = [
  {
    type: 'text',
    content: { text: '你好，我是小梦面试官，由keriko所设计的智能面试官。我已经准备好为您提供专业的面试优化服务。为了开始，请您提供您的简历和应聘职位的要求信息。这将帮助我更好地理解您的背景和应聘职位的具体需求，从而为您提供更加精准的面试准备和反馈。如果您已经准备好这些信息，请上传您的简历文件，我们就可以开始了。如果您有任何疑问或需要帮助，请随时告诉我~' },
    user: { avatar: '//gw.alicdn.com/tfs/TB1DYHLwMHqK1RjSZFEXXcGMXXa-56-62.svg' },
  },
/*  {
    type: 'image',
    content: {
      picUrl: 'https://img1.baidu.com/it/u=997155658,200652501&fm=253&fmt=auto&app=138&f=JPEG?w=500&h=609',
    },
  },*/

];
/*const file = {
  name: 'myFile.zip',
  size: 12345,
};*/

const Welcome: React.FC = () => {
  // 使用useMessages自定义钩子
  const { messages, appendMsg, setTyping } = useMessages(initialMessages);

  const { initialState} = useModel('@@initialState');
  const loading = (
    <span className={`${styles.action} ${styles.account}`}>
      <Spin
        size="small"
        style={{
          marginLeft: 8,
          marginRight: 8,
        }}
      />
    </span>
  );

  if (!initialState) {
    return loading;
  }

  const { currentUser } = initialState;

  if (!currentUser) {
    return loading;
  }

  const handleSend = (type: string, val: string) => {
    if (type === 'text' && val.trim()) {
      appendMsg({
        type: 'text',
        content: { text: val },
        position: 'right',
        user: { avatar: currentUser.avatarUrl },
      });

      setTyping(true);

      setTimeout(() => {
        appendMsg({
          type: 'text',
          content: { text: 'Bala bala' },
          position: 'left', // 假设机器人回复在左侧
          user: { avatar: '//gw.alicdn.com/tfs/TB1DYHLwMHqK1RjSZFEXXcGMXXa-56-62.svg' },
        });
      }, 1000);
    }
  };

  const renderMessageContent = (msg: Message) => {
    const { content } = msg;
    return <Bubble content={content.text} />;
  };

  return (
    <div style={divStyle}>
      <Chat
        /*navbar={{ title: '智能助理' }}*/
        messages={messages}
        renderMessageContent={renderMessageContent}
        onSend={handleSend}
      />
    </div>

  );
};

export default Welcome;
