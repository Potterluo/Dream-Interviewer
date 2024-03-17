# 项目简介
# 完成情况



## 项目介绍

在面试官面试的场景中，面试官希望通过设计合适的问题来探察候选人的岗位匹配程度和实际业务能力，这需要面试官在短时间内对简历内容和岗位要求都能有一些深度理解。
在使用AI工具（coze）进行深入分析面试岗位需求与应聘者简历的基础上，依据既定标准向应聘者提出针对性问题。在应聘者完成问题解答后，将对简历内容及面试表现进行全面评估，并提供具体的改进建议，以助其提升职业竞争力。
- 使用Ant Design Pro 5.X 脚手架搭建项目前端，使用阿里小蜜ChatUI实现消息对话功能
- Java Spring Boot
- MySQL数据库   MyBatis-PLUS
- Redis + Redisson 限流
- RabbitMQ 消息队列
## 需求分析
1.智能对话功能（上传文件）
2.对话存储，删除，一键清空
3.用户登录，注册，信息修改


## 项目架构图
![Pasted image 20240227174149](https://s2.loli.net/2024/03/07/CxtvmaEoq8FWTKe.png)
分布式限流，线程池化，异步化，消息队列等
## 资源
阿里小蜜： https://chatui.io/sdk/getting-started
https://github.com/Otto-J/chatui-chatgpt3.5/tree/main/src
为网站引入动态背景支持  https://hellogithub.com/repository/f53ed895213e4e799043efafe54deda3
## Prompt
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
- 记住，一次只问一个问题。当用户完成该问题回答后，你再选择根据这个问题深挖或者提问新的问题。
- 用户可以选择随时结束面试。无论如何，你都需要提供反馈和建议。
- 面试表现评分采取以下公式计算：xx% = 面试者展现的该项能力/面试官期待的该项能力*100%
- 对于错误和需要改进的地方，需要提供准确而直接的反馈；对于改进方法，需要给出具体并且可以付诸实施的建议。
- 你只能回答用户关于面试和简历修改等相关的问题，不能回答其他问题。
```
# 开发日志
**2024年2月26日**
前端主要功能实现
**2024年2月29日**
待办：后端迁移至Spring Boot万用模板，接口改写
**2024年3月2日**
前端端口改写：请求数据类型位于 @/src/services/ant-design-pro/typing.d.ts
请求接口位于 @config/proxy.ts
**2024年3月5日**
MoonshotAI  API调用实现
**2024年3月6日**
chat调用后端实现，前端惊现离谱bug。Swagger后端调用测试正常，前端调用一直出错。控制台Network已经显示出了返回的包，但是抛出异常了。
**2024年3月7日** 
Configurl.ts 里面定义了只有返回的头部Code不为0，就会抛出异常。修改完成后，初版demo实现。（v0.8）
# 数据库表设计
**用户表user**

| 名称                | 类型 | 备注 |
| ------------------- | ---- | ---- |
| id                  |      | 主键 |
| userAccount         |      | 唯一 |
| userPassword        |      |      |
| userName            |      |      |
| userAvatar          |      |      |
| createTime          |      |      |
| updateTime          |      |      |
| isDelete            |      |      |
**对话列表context**   

| 名称             | 类型 | 备注            |
| ---------------- | ---- | --------------- |
| id               |      | 主键            |
| timeStamp        |      | 时间戳          |
| userAccount      |      | 外键            |
| messageRole      |      | assistant，user |
| messageContent   |      | 内容            |
| listId           |      | 外键            |
| messageModel     |      | 8k,32k,128k     |
| promptTokens     | int  |                 |
| completionTokens | int  |                 |
| totalTokens      | int  |                 |

**会话列表list**

| 名称        | 类型 | 备注 |
| ----------- | ---- | ---- |
| listId      | int  | 主键 |
| createTime  |      |      |
| userAccount |      | 外键 |
| isDelete    |      |      |
|             |      |      |

# 前后端模板食用方法
## 添加新的数据库表实体类操作
- 数据库里面添加表及相应数据
- IDEA连接对应的数据库表，右键选择“MybatisX-Generator”(插件，需要自行安装)
- 根据需要配置需要的选项
  ![Pasted image 20240304193644](https://s2.loli.net/2024/03/07/kojxycBIORrZ654.png)
  可以base package 定位到另外的包（防止出错，影响整个项目），但是每次移动文件时，xml的内容并不会自动更新。
  注意service，./resource/mapper/xxxxxMapper.xml 里面的各种引用是否正确，xml引号里面的引用可能不会自动更改。
  ![Pasted image 20240304194122](https://s2.loli.net/2024/03/07/btdrsMRm9Yx8QKZ.png)
- entity的注解
@TableId(type = IdType.ASSIGN_ID)  
private Integer id;
主键ID（随机)

@TableLogic  
private Integer isDelete;
逻辑删除字段
- service里面实现需要的各种方法（service定义，impl实现）
- 通义灵码可以直接生成service的单元测试代码
- 对于要使用前端传回对象的情况，可以在DTO层定义一个类是实现
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
## 会话接口实现
为了实现会话的上下文保持，你需要在服务器端存储每次请求的上下文信息。这样，当用户再次发起请求时，你可以从服务器端获取之前的对话内容，并将其作为新的请求的一部分。以下是一个简化的示例，展示了如何在Spring Boot Controller中实现这一功能：
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
## 前端调用接口自动生成
openapi 规范 （接口文档规范）
后端模板使用的swagger的openai版本为v2，与前端不兼容，导致出错。升级后恢复。
## chat调用流程（遇到的困难）
第一次完整地自己做一个非CURD的完整逻辑，还是有一些收获的。
再一次认识到了面向对象的重要性。
用户chat的设计流程：
- 1
对于不同接口直接的调用，需要设计不同的类来实现

对于Service chat方法的实现，把上述全部功能方法一个方法里过于冗杂。可以实现一些私有方法，并不对外提供，来实现这些小功能。
**对象过多，把自己绕晕了**
在流程和对象设计好之后记得画用例图，把所有的类已经各种方法的参数及返回一一标注对照来写。
如下（不规范版）：
![Chat调用过程 (1)](https://s2.loli.net/2024/03/07/a59bs3ZSLflkwY1.png)

# 效果预览
## 使用coze在discord上的效果
![Pasted image 20240226214842](https://s2.loli.net/2024/02/26/jq74OkJsG1lRUfI.png)
## ChatUI界面初步实现
![a7030bad120ce559187434b302d1a4b](https://s2.loli.net/2024/02/26/q7B3CmtlApMfYxz.png)
## 连续对话功能
![](https://pic.imgdb.cn/item/65eabc9e9f345e8d03a7d636.png)
