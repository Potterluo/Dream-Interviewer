import '@chatui/core/es/styles/index.less';
import React from 'react';
// 引入组件
import Chat, { Bubble, useMessages } from '@chatui/core';
// 引入样式
import { Button } from 'antd';
import '../../../../init/yupi-antd-frontend-init-master/src/pages/chatui-theme.css';

import { chat } from '@/services/backend/contextController';
import { Link, useModel } from '@@/exports';
import { FloatButton } from 'antd';

// 定义消息类型
interface Message {
  type: 'text';
  content: { text: string };
  position?: 'left' | 'right';
}
const divStyle: React.CSSProperties = {
  position: 'absolute',
  width: '80%',
  height: '88vh', // 设置高度为100%以填充父元素的高度
  left: '50%', // 设置左边距为50%
  transform: 'translateX(-50%)', // 使用transform将元素水平居中
};
const initialMessages = [
  {
    type: 'text',
    content: {
      text: '你好，我是小梦面试官，由keriko所设计的智能面试官。我已经准备好为您提供专业的面试优化服务。为了开始，请您提供您的简历和应聘职位的要求信息。这将帮助我更好地理解您的背景和应聘职位的具体需求，从而为您提供更加精准的面试准备和反馈。如果您已经准备好这些信息，请上传您的简历文件，我们就可以开始了。如果您有任何疑问或需要帮助，请随时告诉我~',
    },
    user: { avatar: '//gw.alicdn.com/tfs/TB1DYHLwMHqK1RjSZFEXXcGMXXa-56-62.svg' },
  },
  /*    {
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
/*async function newMessageList({ userAccount }: { userAccount: string }) {
  try {
    const listId = await addMessageList(userAccount);
    // 处理 listId
  } catch (error) {
    // 处理错误
  }
}*/

/*async function sendMessage({ contextRequest }: { contextRequest: ContextRequest }) {
  try {
    // 发起请求并等待响应
    const response = await chat(contextRequest);
    // 等待响应体转换为JSON

    const data = response.data.messageContent();

    if (!response.ok) {
      // 处理非2xx的响应
      throw new Error(data.message || '请求失败');
    }

    return data; // 返回数据供其他操作使用
  } catch (error) {
    console.error('接口调用出错:', error);
    // 可以在这里处理错误，比如显示错误消息
  }
}*/

const ChatPage: React.FC = () => {
  // 使用useMessages自定义钩子
  const { messages, appendMsg, setTyping } = useMessages(initialMessages);
  const { initialState } = useModel('@@initialState');
  const { currentUser } = initialState || {};
  if (!currentUser) {
    return (
      <Link to="/user/login">
        <Button type="primary" shape="round">
          登录
        </Button>
      </Link>
    );
  }
  /*  const listId = await addMessageList(currentUser.userName);
  函数不是异步的：await 只能用于等待返回 Promise 对象的异步函数。如果 addMessageList 函数没有返回 Promise，或者不是异步的，那么使用 await 就会导致错误。

不在异步函数中使用：await 必须在 async 函数内部使用。如果你尝试在一个普通的函数或者全局作用域中使用 await，也会报错。*/
  //newMessageList({ userAccount: currentUser.userName });
  async function sendMessage(requestBody: ContextRequest) {
    try {
      const response = await chat(requestBody);

      // 返回响应中的messageContent属性值
      return response.messageContent;
    } catch (error) {
      // 处理错误
      console.error('Error fetching data:', error);
      throw error; // 可以选择抛出错误，也可以根据实际情况处理错误逻辑
    }
  }
  const handleSend = (type: string, val: string) => {
    if (type === 'text' && val.trim()) {
      appendMsg({
        type: 'text',
        content: { text: val },
        position: 'right',
        user: { avatar: currentUser.userAvatar },
      });

      const requestBody: ContextRequest = {
        userAccount: 'admin',
        messageRole: 'user',
        messageContent: val,
        listId: 1236346234,
      };

      /*      appendMsg({
        type: 'text',
        content: { text: {messageContent} },
        position: 'left', // 假设机器人回复在左侧
        user: { avatar: '//gw.alicdn.com/tfs/TB1DYHLwMHqK1RjSZFEXXcGMXXa-56-62.svg' },
      });*/

      setTimeout(() => {
        sendMessage(requestBody)
          .then((messageContent) => {
            console.log('Response Message Content:', messageContent);
            appendMsg({
              type: 'text',
              content: { text: messageContent },
              position: 'left', // 假设机器人回复在左侧
              user: { avatar: '//gw.alicdn.com/tfs/TB1DYHLwMHqK1RjSZFEXXcGMXXa-56-62.svg' },
            });
          })
          .catch((error) => {
            // 处理错误
            console.error('调用服务出错:', error);
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
      <FloatButton onClick={() => console.log('onClick')} />
      <Chat
        /* navbar={{ title: '智能助理' }}*/
        messages={messages}
        // @ts-ignore
        renderMessageContent={renderMessageContent}
        onSend={handleSend}
      />
      <div
        style={{
          display: 'flex',
          justifyContent: 'center',
          color: '#808080',

          /*        justify-content: center; /!* 水平居中 *!/
        align-items: center; /!* 垂直居中 *!/
        text-align: center; /!* 文字居中 *!/
        color: #808080; /!* 设置文字颜色为灰色 *!/*/
        }}
      >
        <span>
          <p>内容由 AI 大模型生成，请仔细甄别 &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;</p>
        </span>
        <span>
          <a href={'https://kimi.moonshot.cn/'}>MoonshotAI提供支持</a>
        </span>
      </div>
    </div>
  );
};

export default ChatPage;
