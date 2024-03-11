import { chat } from '@/services/backend/contextController';
import { uploadFile } from '@/services/backend/cvFileController';
import { Link, useModel } from '@@/exports';
import Chat, { Bubble, useMessages } from '@chatui/core';
import '@chatui/core/es/styles/index.less';
import { Button, FloatButton, GetProp, message, Tooltip, Upload, UploadProps } from 'antd';
import React from 'react';
import '../../../../init/yupi-antd-frontend-init-master/src/pages/chatui-theme.css';

// import ContextRequest = API.ContextRequest; 一引入就报错，不引入也能用，就很奇怪

// 定义消息类型
interface Message {
  type: 'text';
  content: { text: string };
  position?: 'left' | 'right';
}
const divStyle: React.CSSProperties = {
  position: 'absolute',
  width: '80vw',
  height: '85vh', // 设置高度为100%以填充父元素的高度
  left: '50%', // 设置左边距为50%
  transform: 'translateX(-50%)', // 使用transform将元素水平居中
};
const initialMessages = [
  {
    type: 'text',
    content: {
      text: '你好，我是小梦面试官，由keriko所设计的智能面试官。我已经准备好为您提供专业的面试优化服务。为了开始，请您提供您的简历和应聘职位（JD）的要求信息。这将帮助我更好地理解您的背景和应聘职位的具体需求，从而为您提供更加精准的面试准备和反馈。如果您已经准备好这些信息，请上传您的简历文件，我们就可以开始了。如果您有任何疑问或需要帮助，请随时告诉我~',
    },
    user: { avatar: '//gw.alicdn.com/tfs/TB1DYHLwMHqK1RjSZFEXXcGMXXa-56-62.svg' },
  },
  {
    type: 'text',
    content: {
      text: '请输入你想应聘的岗位及要求，点击页面右下角图标上传简历开始。',
    },
    user: { avatar: '//gw.alicdn.com/tfs/TB1DYHLwMHqK1RjSZFEXXcGMXXa-56-62.svg' },
  },
];
type FileType = Parameters<GetProp<UploadProps, 'beforeUpload'>>[0];
const allowedTypes = [
  'application/pdf',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  'application/msword',
  'text/plain',
];
const beforeUpload = (file: FileType) => {
  const isAllowedType = allowedTypes.includes(file.type);
  const isSizeValid = file.size / 1024 / 1024 < 10; // 10MB

  if (!isAllowedType) {
    message.error('你必须上传 PDF, DOC, DOCX, TXT 等类型的简历!');
  }
  if (!isSizeValid) {
    message.error('简历文件必须小于10M！');
  }

  return isAllowedType && isSizeValid;
};

const ChatPage: React.FC = () => {
  // 使用useMessages自定义钩子
  const { messages, appendMsg } = useMessages(initialMessages);
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
  // @ts-ignore
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
  const props: UploadProps = {
    name: 'cvfile',
    //TODO 开发访问地址
    /*action: 'http://localhost:8101/api/upload',*/
    showUploadList: true,
    maxCount: 1,
    disabled: false,
    beforeUpload: beforeUpload,
    customRequest: uploadCvFile,
    headers: {
      authorization: 'authorization-text',
    },
    onChange(info) {
      /*      if (info.file.status !== 'uploading') {
        console.log(info.file, info.fileList);
      }*/
      if (info.file.status === 'done') {
        message.success(`${info.file.name} file uploaded successfully`);
        appendMsg({
          type: 'text',
          position: 'left', // 假设机器人回复在左侧
          content: {
            text: `${info.file.name}上传成功，简历已收到。`,
          },
          user: { avatar: '//gw.alicdn.com/tfs/TB1DYHLwMHqK1RjSZFEXXcGMXXa-56-62.svg' },
        });
      } else if (info.file.status === 'error') {
        message.error(`${info.file.name} file upload failed.`);
        appendMsg({
          type: 'text',
          position: 'left', // 假设机器人回复在左侧
          content: {
            text: `${info.file.name}上传失败。`,
          },
          user: { avatar: '//gw.alicdn.com/tfs/TB1DYHLwMHqK1RjSZFEXXcGMXXa-56-62.svg' },
        });
      }
    },
  };
  async function uploadCvFile(option: { file: any }) {
    const file = option.file as File;

    // 如果文件类型和大小都符合要求，调用后端上传接口
    try {
      const formData = new FormData();
      formData.append('cvfile', file); // 假设后端期望文件字段名为'file'
      const requestFileBody: ContextRequest = {
        userAccount: 'admin',
        messageRole: 'user',
        messageContent: '',
        listId: 81097580,
      };
      console.log('开始上传');
      appendMsg({
        type: 'text',
        content: { text: `${file.name}` },
        position: 'right',
        user: {
          avatar:
            currentUser.userAvatar ?? 'https://pic.imgdb.cn/item/65eae1529f345e8d03011b3b.png',
        },
      });
      const cvcontent = await uploadFile(requestFileBody, formData);
      const requestCvBody: ContextRequest = {
        userAccount: 'admin',
        messageRole: 'user',
        messageContent: cvcontent,
        listId: 81097580,
      };
      const response = await chat(requestCvBody);
      console.log(response);
      appendMsg({
        type: 'text',
        content: { text: response.messageContent },
        position: 'left',
        user: { avatar: '//gw.alicdn.com/tfs/TB1DYHLwMHqK1RjSZFEXXcGMXXa-56-62.svg' },
      });
    } catch (error) {
      console.error('Error uploading file:', error);
      throw error; // 可以选择抛出错误，也可以根据实际情况处理错误逻辑
    }
  }
  const handleSend = (type: string, val: string) => {
    if (type === 'text' && val.trim()) {
      appendMsg({
        type: 'text',
        content: { text: val },
        position: 'right',
        user: {
          avatar:
            currentUser.userAvatar ?? 'https://pic.imgdb.cn/item/65eae1529f345e8d03011b3b.png',
        },
      });

      // @ts-ignore
      const requestBody: ContextRequest = {
        userAccount: 'admin',
        messageRole: 'user',
        messageContent: val,
        listId: 81097580,
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

  const onFloatClick = () => {};

  return (
    <div style={divStyle}>
      {/*      <Modal title="简历上传" open={isModalOpen} onOk={handleOk} onCancel={handleCancel}>

          <Button icon={<UploadOutlined />}>Click to upload</Button>

      </Modal>*/}
      <Chat
        /* navbar={{ title: '智能助理' }}*/
        messages={messages}
        // @ts-ignore
        renderMessageContent={renderMessageContent}
        onSend={handleSend}
      />
      <Upload {...props}>
        <Tooltip title="简历上传">
          <FloatButton onClick={onFloatClick} />
        </Tooltip>
      </Upload>

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
