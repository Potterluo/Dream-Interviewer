# Dream-Interviewer
# 项目简介
## 项目介绍
在面试官面试的场景中，面试官希望通过设计合适的问题来探察候选人的岗位匹配程度和实际业务能力，这需要面试官在短时间内对简历内容和岗位要求都能有一些深度理解。
在使用AI工具（coze）进行深入分析面试岗位需求与应聘者简历的基础上，依据既定标准向应聘者提出针对性问题。在应聘者完成问题解答后，将对简历内容及面试表现进行全面评估，并提供具体的改进建议，以助其提升职业竞争力。
- 使用Ant Design Pro 5.X 脚手架搭建项目前端，使用阿里小蜜ChatUI实现消息对话功能

- Java Spring Boot

- MySQL数据库   MyBatis-PLUS

- Redis + Redisson 限流

- RabbitMQ 消息队列

  

## prompt

```
# 角色
你是一个娴熟的面试优化专家，因精湛的技巧而获得"小云面试官"的称号。你的主要职责是详细地评估和分析中文用户的面试表现和应聘职位的要求，你提供的高效反馈能帮助求职者增强他们的面试技巧和表现。

## 技能
### 技能1：职位要求掌握
- 从用户提供的职位描述（JD）中提取关键信息。
- 使用Google搜索同类职位的要求，找出匹配度超过99%的招聘信息。
- 深入地分析和理解职位的需求和要求。

### 技能2：模拟面试提问
- 根据职位描述和要求，挑选出符合专业相关的问题进行询问。
- 深入挖掘用户提供的简历，根据用户回答情况采用追问形式。
- 根据宝洁八问等常见面试方式，在对用户的性格、团队协作等方面进行提问。

### 技能3：面试评分
- 掌握并识别各种面试类型（如：结构化面试、行为面试、情景模拟面试、STAR法则面试等）。
- 在评估面试者的表现时，需要结合到具体的面试情境和需求进行个性化的评分。

### 技能4：反馈和建议
- 公正但直接地对面试者在面试中的失误和不利行为进行评分和反馈。
- 如果面试得分低于60%，需要指出面试者在面试过程中的重大错误，并注意观察面试者对反馈的接受程度。

## 规定
- 在内容分析中需包括以下六大指标：应聘能力的匹配度、思维过程和积极性、解决问题的能力、沟通技巧、价值观和理想，以及诚实度。
- 在反馈中需尽可能地采用专业而人性化的语言，让用户在一个轻松友好的环境中接受专业而具体的反馈。

### 输出格式
- 📊 应聘能力匹配度：<百分比>
   解释：<说明>
- 🧠 思维过程和积极性：<百分比>
   解释：<说明>
- 💡 解决问题的能力：<百分比>
   解释：<说明>
- 📣 沟通技巧：<百分比>
   解释：<说明>
- 🎯 价值观和理想：<百分比>
   解释：<说明>
- 📈 预测HR的满意度：<百分比>
- 详尽的分析：<文章>
- 优点:<列表>
- 简历需要改进的地方：<列表>
- 需要改进的地方：<列表>
- 改进策略：<列表>

## 约束
- 用户必须提供简历（PDF，纯文本等格式），并尽可能提供职位要求信息，以便进行全面的分析。如果信息不足，应向用户索取。
- 你需要逐一向用户提供8-15个问题，一次输出一个问题，待用户回答完毕后再输出下一个问题，面试过程中不进行评价。你需要根据用户的回答情况适当地调整问题，可以对回答地内容进行深度提问。在用户完成所有的回答后，你需要立即提供反馈和建议。
- 用户可以选择随时结束面试。无论如何，你都需要提供反馈和建议。
- 面试表现评分采取以下公式计算：xx% = 面试者展现的该项能力/面试官期待的该项能力*100%
- 对于错误和需要改进的地方，需要提供准确而直接的反馈；对于改进方法，需要给出具体并且可以付诸实施的建议。
```



## prompt效果预览

![Pasted image 20240226214842](https://s2.loli.net/2024/02/26/jq74OkJsG1lRUfI.png)

# 项目技术栈

## 所用知识

- 使用Ant Design Pro 5.X 脚手架搭建项目前端，使用阿里小蜜ChatUI实现消息对话功能

- Java Spring Boot

- MySQL数据库   MyBatis-PLUS

- Redis + Redisson 限流

- RabbitMQ 消息队列

## 脚手架资源

MoonShotAI 开放平台 https://platform.moonshot.cn/docs

阿里小蜜ChatUI  https://chatui.io/sdk/getting-started

Ant Design Pro 前端框架 https://pro.ant.design/

# 遇到的问题
## ant design pro框架更改
Header 头部组件一直未找到：通过阅读官方文档，定位app.tsx，
用户名、头像信息可以通过配置全局初始化信息来提供数据。
```ts
// src/app.tsx
import React from 'react';
export const layout = {
  rightRender: (initialState, setInitialState) => {
    // xxx
    return 'xxx';
  },
};
```
`rightRender` 才为真正的导航栏右侧组件

这个SettingDrawer在文档里说只在开发模式下出现, 但是我用的这个版本打包部署到生产环境仍然会显示, 需要/src/app.tsx 中添加下面这一行让它只在开发环境中出现

## 获取当前用户
```ts
import {useModel} from "@@/plugin-model/useModel";

const { initialState} = useModel('@@initialState');  
const loading = (  
  <span className={`${styles.action} ${styles.account}`}>  
    <Spin      size="small"  
      style={{  
        marginLeft: 8,  
        marginRight: 8,  
      }}  
    />  
  </span>);  
  
if (!initialState) {  
  return loading;  
}  
  
const { currentUser } = initialState;  
  
if (!currentUser) {  
  return loading;  
}
```
ant design在登录时就获取了当前用户得信息，通过这些语句，就可以在当前页面使用currentUser及其属性值。（找猫画瓢出来，并未实现）
## 主题配置相关
这个SettingDrawer在文档里说只在开发模式下出现, 但是我用的这个版本打包部署到生产环境仍然会显示, 需要/src/app.tsx 中添加下面这一行让它只在开发环境中出现。
```ts
{(process.env.NODE_ENV === "development")&&!props.location?.pathname?.includes('/login') && (  
  <SettingDrawer  
    enableDarkTheme  
    settings={initialState?.settings}  
    onSettingChange={(settings) => {  
      setInitialState((preInitialState) => ({  
        ...preInitialState,  
        settings,  
      }));  
    }}
```
想设置深色模式切换的按钮，`darkreader`未生效，只放了按钮。不影响整体项目，先搁置。
![a7030bad120ce559187434b302d1a4b](https://s2.loli.net/2024/02/26/q7B3CmtlApMfYxz.png)



