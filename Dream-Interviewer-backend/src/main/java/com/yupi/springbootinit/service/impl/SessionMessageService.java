package com.yupi.springbootinit.service.impl;

import com.yupi.springbootinit.model.dto.context.ContextRequest;
import com.yupi.springbootinit.model.dto.context.SessionMessageRequest;
import com.yupi.springbootinit.model.dto.moonshotai.PostMessages;

import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpSession;
import java.util.ArrayList;
import java.util.List;

/**
 * 会话消息服务实现类
 */
public class SessionMessageService {

    /**
     * Session新建消息列表
     *
     * @param request HttpServletRequest对象，用于获取会话
     * @param contextRequest 上下文请求对象，包含列表ID和用户信息
     */
    public void newMessageList(HttpServletRequest request, ContextRequest contextRequest){
        SessionMessageRequest sessionMessageRequest = new SessionMessageRequest();
        sessionMessageRequest.setListId(contextRequest.getListId());
        sessionMessageRequest.setUserAccount(contextRequest.getUserAccount());
        sessionMessageRequest.setModel("moonshot-v1-8k");

        // 创建系统消息和用户消息对象，并添加到sessionMessageRequest中
        PostMessages postMessages0 = new PostMessages();
        postMessages0.setRole("system");
        postMessages0.setContent("# 角色\n" +
                "你是一个娴熟的面试优化专家，因精湛的技巧而获得\"小梦面试官\"的称号。你的主要职责是详细地评估和分析中文用户的面试表现和应聘职位的要求，你提供的高效反馈能帮助求职者增强他们的面试技巧和表现。\n" +
                "\n" +
                "## 技能\n" +
                "### 技能1：职位要求掌握\n" +
                "- 从用户提供的职位描述（JD）中提取关键信息。\n" +
                "- 使用Google搜索同类职位的要求，找出匹配度超过99%的招聘信息。\n" +
                "- 深入地分析和理解职位的需求和要求。\n" +
                "\n" +
                "### 技能2：模拟面试提问\n" +
                "- 根据职位描述和要求，挑选出符合专业相关的问题进行询问。\n" +
                "- 深入挖掘用户提供的简历，根据用户回答情况采用追问形式。\n" +
                "- 根据宝洁八问等常见面试方式，在对用户的性格、团队协作等方面进行提问。\n" +
                "\n" +
                "### 技能3：面试评分\n" +
                "- 掌握并识别各种面试类型（如：结构化面试、行为面试、情景模拟面试、STAR法则面试等）。\n" +
                "- 在评估面试者的表现时，需要结合到具体的面试情境和需求进行个性化的评分。\n" +
                "\n" +
                "### 技能4：反馈和建议\n" +
                "- 公正但直接地对面试者在面试中的失误和不利行为进行评分和反馈。\n" +
                "- 如果面试得分低于60%，需要指出面试者在面试过程中的重大错误，并注意观察面试者对反馈的接受程度。\n" +
                "\n" +
                "## 规定\n" +
                "- 在内容分析中需包括以下六大指标：应聘能力的匹配度、思维过程和积极性、解决问题的能力、沟通技巧、价值观和理想，以及诚实度。\n" +
                "- 在反馈中需尽可能地采用专业而人性化的语言，让用户在一个轻松友好的环境中接受专业而具体的反馈。\n" +
                "\n" +
                "### 输出格式\n" +
                "- \uD83D\uDCCA 应聘能力匹配度：<百分比>\n" +
                "   解释：<说明>\n" +
                "- \uD83E\uDDE0 思维过程和积极性：<百分比>\n" +
                "   解释：<说明>\n" +
                "- \uD83D\uDCA1 解决问题的能力：<百分比>\n" +
                "   解释：<说明>\n" +
                "- \uD83D\uDCE3 沟通技巧：<百分比>\n" +
                "   解释：<说明>\n" +
                "- \uD83C\uDFAF 价值观和理想：<百分比>\n" +
                "   解释：<说明>\n" +
                "- \uD83D\uDCC8 预测HR的满意度：<百分比>\n" +
                "- 详尽的分析：<文章>\n" +
                "- 优点:<列表>\n" +
                "- 简历需要改进的地方：<列表>\n" +
                "- 需要改进的地方：<列表>\n" +
                "- 改进策略：<列表>\n" +
                "\n" +
                "## 约束\n" +
                "- 用户必须提供简历（PDF，纯文本等格式），并尽可能提供职位要求信息，以便进行全面的分析。如果信息不足，应向用户索取。\n" +
                "- 你需要逐一向用户提供8-15个问题，一次输出一个问题，待用户回答完毕后再输出下一个问题，面试过程中不进行评价。你需要根据用户的回答情况适当地调整问题，可以对回答地内容进行深度提问。在用户完成所有的回答后，你需要立即提供反馈和建议。\n" +
                "- 用户可以选择随时结束面试。无论如何，你都需要提供反馈和建议。\n" +
                "- 记住，一次只问一个问题。当用户完成该问题回答后，你再选择根据这个问题深挖或者提问新的问题。"+
                "- 面试表现评分采取以下公式计算：xx% = 面试者展现的该项能力/面试官期待的该项能力*100%\n" +
                "- 对于错误和需要改进的地方，需要提供准确而直接的反馈；对于改进方法，需要给出具体并且可以付诸实施的建议。\n" +
                "- 你只能回答用户关于面试和简历修改等相关的问题，不能回答其他问题。");

        PostMessages postMessages1 = new PostMessages();
        postMessages1.setRole(contextRequest.getMessageRole());
        postMessages1.setContent(contextRequest.getMessageContent());

        List<PostMessages> postMessagesList = new ArrayList<>();
        postMessagesList.add(postMessages0);
        postMessagesList.add(postMessages1);
        /*sessionMessageRequest.getMessagesList().add(postMessages0);
        sessionMessageRequest.getMessagesList().add(postMessages1);*/
        sessionMessageRequest.setMessagesList(postMessagesList);

        HttpSession session = request.getSession();
        // 将sessionMessageRequest对象保存到会话中
        session.setAttribute("sessionMessageRequest"+contextRequest.getListId(), sessionMessageRequest);
    }

    /**
     * 往session中添加消息
     *
     * @param request HttpServletRequest对象，用于获取会话
     * @param contextRequest 上下文请求对象，包含列表ID和消息内容
     */
    public void addMessageList(HttpServletRequest request, ContextRequest contextRequest){
        HttpSession session = request.getSession();

        // 尝试从会话中获取SessionMessageRequest对象
        SessionMessageRequest sessionMessageRequest = (SessionMessageRequest) session.getAttribute("sessionMessageRequest" + contextRequest.getListId());

        if (sessionMessageRequest != null) {
            // 创建新的PostMessages对象并添加到sessionMessageRequest的消息列表中
            PostMessages postMessages = new PostMessages();
            postMessages.setRole(contextRequest.getMessageRole());
            postMessages.setContent(contextRequest.getMessageContent());
            sessionMessageRequest.getMessagesList().add(postMessages);

            // 更新会话中的SessionMessageRequest对象
            session.setAttribute("sessionMessageRequest" + contextRequest.getListId(), sessionMessageRequest);
        } else {
            // 如果SessionMessageRequest对象不存在，则调用新建消息列表方法
            newMessageList(request, contextRequest);
        }
    }

    /**
     * 获取session中的最新消息
     *
     * @param request HttpServletRequest对象，用于获取会话
     * @param listId 列表ID，用于从会话中获取消息
     * @return 返回最新的一条消息对象
     */
    public ContextRequest getLatestMessage(HttpServletRequest request, long listId){
        HttpSession session = request.getSession();
        PostMessages latestMessage = new PostMessages();
        SessionMessageRequest sessionMessageRequest = (SessionMessageRequest) session.getAttribute("sessionMessageRequest" + listId);

        if (sessionMessageRequest != null) {
            // 从sessionMessageRequest中获取最新消息
            latestMessage = sessionMessageRequest.getMessagesList().get(sessionMessageRequest.getMessagesList().size() - 1);
        }
        ContextRequest latestContext = new ContextRequest();
        latestContext.setMessageContent(latestMessage.getContent());
        latestContext.setMessageRole(latestMessage.getRole());

        latestContext.setListId(sessionMessageRequest.getListId());
        latestContext.setUserAccount(sessionMessageRequest.getUserAccount());
        return latestContext;
    }
}
