# 完成情况

- [x] 登录注册功能
- [x] 退出登录
- [x] 管理员用户管理
- [x] 调用AI接口进行连续对话 （部署部分有问题）
- [x] 上传并解析简历
- [x] 根据prompt对用户进行提问并评价（偶尔出现不能逐一提问，提前暴露评价模板等）
- [ ] Redis-session全部改为Redis存储
- [ ] 用户自定义个人信息修改
- [ ] 面试列表维护，历史记录查看
- [ ] 简历解析异步化，流式对话
- [ ] 令牌桶、多key轮换策略



# 项目简介



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
## 开发资源
- 阿里小蜜ChatUI： https://chatui.io/sdk/getting-started
  	https://github.com/Otto-J/chatui-chatgpt3.5/tree/main/src

- 为网站引入动态背景支持：  https://hellogithub.com/repository/f53ed895213e4e799043efafe54deda3

- MoonShot API开放平台：https://platform.moonshot.cn/docs/docs

   火山引擎大模型服务平台：https://www.volcengine.com/docs/82379/1158281#java

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

后端迁移至Spring Boot万用模板，接口改写

**2024年3月2日**
前端端口改写：请求数据类型位于 @/src/services/ant-design-pro/typing.d.ts
请求接口位于 @config/proxy.ts

**2024年3月5日**
MoonshotAI  API调用实现

**2024年3月6日**
chat调用后端实现，前端惊现离谱bug。Swagger后端调用测试正常，前端调用一直出错。控制台Network已经显示出了返回的包，但是抛出异常了。

**2024年3月7日** 
Configurl.ts 里面定义了只有返回的头部Code不为0，就会抛出异常。修改完成后，初版demo实现。 http://keriko.fun/index.php/archives/36/

**2024年3月11日**
简历上传并解析功能实现 。http://keriko.fun/index.php/archives/36/

**2024年3月15日** 
主体功能实现，简历、JD解析，连续提问，面试评价 http://keriko.fun/index.php/archives/43/

**2024年3月16日**
项目部署上线。（连续会话实现，每次请求都会新建session）

**2024年3月17日**
api请求地址与密钥全局封装统一；文件路径修改（windows：\      Linux  /)







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
```typescript
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
## chat调用流程
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

## 文件上传接口不对

端使用 Ant Design 提供的upload 组件。
（经验：先使用最简单的组件，需要什么慢慢往上加。不然真的头秃。）

```javascript
const props: UploadProps = {  
  name: 'cvfile',  
  //TODO 开发访问地址  
  action: 'http://localhost:8101/api/upload', 
  showUploadList: false,  
  maxCount: 1, //这样实际不生效
  disabled: false, //可以通过设置其熟悉来限制一次会话只上传一次信息
  headers: {  
    authorization: 'authorization-text',  
  },  
  onChange(info) {  
  if (info.file.status === 'done') {  
      message.success(`${info.file.name} file uploaded successfully`);  
      });  
    } else if (info.file.status === 'error') {  
      message.error(`${info.file.name} file upload failed.`);  
    }  
  },  
};

//悬浮球效果
<Upload {...props} >  
  <Tooltip title="简历上传">  
    <FloatButton onClick={onFloatClick} />  
  </Tooltip></Upload>
```

[![Pasted image 20240311215218](https://s2.loli.net/2024/03/11/9hDVaElqUmzZ4Iv.png)](https://s2.loli.net/2024/03/11/9hDVaElqUmzZ4Iv.png)
后端实现

```java
@PostMapping("/upload")  
public ResponseEntity<String> uploadFile(@RequestParam("cvfile")  MultipartFile file) {  
    if (file.isEmpty()) {  
        return new ResponseEntity<>("请选择文件", HttpStatus.BAD_REQUEST);  
    }  
  
    try {  
        String fileName = file.getOriginalFilename();  
        // 设置文件存储路径  
        Path path = Paths.get(uploadPath, fileName);  
        // 保存文件  
        Files.copy(file.getInputStream(), path);  
        return new ResponseEntity<>("文件上传成功: " + fileName, HttpStatus.OK);  
    } catch (Exception e) {  
        e.printStackTrace();  
        return new ResponseEntity<>("文件上传失败", HttpStatus.INTERNAL_SERVER_ERROR); 
    }
}
```

**JAVA**

这样就能实现用户的简历上传功能。
前期把前端调用后端的接口写错了（/file/upload），其为模板定义的头像上传接口。一直报请求参数错误，直到把Swagger就回来才发现请求错了接口。（是我错怪Ant design了）

**后续**

由于官方结构只传递了文件，后端无法获取当前用户信息（其实也行？）

改写接口

customRequest	通过覆盖默认的上传行为，可以自定义自己的上传实现	function

```typescript
async function uploadCvFile(option: { file: any }) {
    const file = option.file as File;

    // 如果文件类型和大小都符合要求，调用后端上传接口
    try {
      const formData = new FormData();
      formData.append('cvfile', file); // 假设后端期望文件字段名为'file'
      // @ts-ignore
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
            // @ts-ignore
            currentUser.userAvatar ?? 'https://pic.imgdb.cn/item/65eae1529f345e8d03011b3b.png',
        },
      });
      const cvcontent = await uploadFile(requestFileBody, formData);
      // @ts-ignore
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
```

**TODO 异步化改写** （用户等待时间过长）

## apiKey等常量不能全局生效

在配置文件application.yml文件中，写入需要使用的全局变量

```properties
file:  
  upload:  
    path: E:\TestUpload
```

application.properties写法

```
file.upload.path: E:\TestUpload
```

然后创建一个实体类，在实体类的属性中直接用 **@Value** 注解获取配置文件中的常量

```java
import org.springframework.beans.factory.annotation.Value;  
import org.springframework.stereotype.Component;  
  
@Component  
public class FilePath {  
    public String getUploadPath() {  
        return uploadPath;  
    }  
  
    public void setUploadPath(String uploadPath) {  
        this.uploadPath = uploadPath;  
    }  
  
    @Value("${file.upload.path}")  
    String uploadPath;  
}
```

在使用的地方 用 **@Autowired** 注解实例化content对象，然后直接content.get属性 即可获得常量。例子如下：

```java
@Autowired  
    private FilePath filePath;

@RequestMapping("/test")
    public  String tt(){
        System.out.println("常量path:"+filePath.getUploadPath());
        return null;
    }
```

### 装配不生效 （常量对象为null）

一、在静态变量上使用失效

第一种情况是在静态变量上无法使用注解，@Autowire注解自然失效。虽然注入时不会报错，但是当运行时期使用注入的对象时会报空指针异常。

```java
public class demoUtils {
    //此时虽然不报错，运行时汇报空指针异常
    @Autowired
    private static User user;
    public static void test(){
        user.setName("zql");
        System.out.println(user.getName());
    }
}
```

这时就需要new对象，每次都会创建一个对象，在多线程中可能会出现问题，此时可以使用单例模式解决。

```java
public class DemoUtils {

    /*@Autowired
    private static User user;*/

    private static User user = new User();

    public static void test(){
        user.setName("zql");
        System.out.println(user.getName());
    }
}
```

二、new的对象中使用@Autowire注入其他类

如果有 A，B，C 三个类，在 A 中 new B()，在 B 中使用@Autowire注入 C，此时的@Autowire注解也会失效，C无法注入，运行时报空指针异常。以下是测试。

相当于 A 类。

```java
@RestController
public class AController {

    BService bService = new BService();

    @GetMapping("/test")
    public String test(){
        bService.test();
        return "正常执行成功";
    }
}
```

相当于 B 类，在注入时正常，运行时报空指针异常。

```java
@Service
public class BService {
    //注解失效，运行时会出现空指针异常
    @Autowired
    CService cService;
    public void test(){
        cService.testHello();
    }
}
```

相当于 C 类。

```java
@Service
public class CService {
    public void testHello(){
        System.out.println("hello,world");
    }
}
```

**类比思考** 

一般常在 XXXServiceImpl中来使用常量，new Service的时候并不会new impl，因此常量可以正常使用。
但若是自己写的Service，若直接在Service中注入了另外一个类，而且还通过new使用了，就会报空指针异常错误。

解决

解决方法：在 A 类中使用@Autowire注解来注入 B 。如下所示：

```java
@RestController
public class AController {

    //BService bService = new BService();
    @Autowired
    BService bService;

    @GetMapping("/test")
    public String test(){
        bService.test();
        return "正常执行成功";
    }
}
```

当然在 B 中将@Autowire注解注入C，改成new C()也可以（不推荐）。

总的来说：使用@Autowire注解注入，都使用注解注入（推荐，生成的对象交给了Spring管理）；使用new，都使用new（不推荐）。
## 基于Session实现的上下文存储在服务器上失效

- 暂未解决
  

# 效果预览
## 使用coze在discord上的效果
![Pasted image 20240226214842](https://s2.loli.net/2024/02/26/jq74OkJsG1lRUfI.png)
## ChatUI界面初步实现
![a7030bad120ce559187434b302d1a4b](https://s2.loli.net/2024/02/26/q7B3CmtlApMfYxz.png)
## 连续对话功能

![](https://pic.imgdb.cn/item/65eabc9e9f345e8d03a7d636.png)

## 主体功能实现

![小梦面试官效果预览](https://s2.loli.net/2024/03/16/vusSOYEDxQoAXFk.png)
