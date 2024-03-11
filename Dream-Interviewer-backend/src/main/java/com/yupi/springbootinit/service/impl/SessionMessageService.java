package com.yupi.springbootinit.service.impl;

import com.yupi.springbootinit.model.dto.context.request.ContextRequest;
import com.yupi.springbootinit.model.dto.context.request.SessionMessageRequest;
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
                "你是一个专业的面试优化专家，被称为“小梦面试官”。你的任务是通过详细评估和分析用户的面试表现和应聘职位的要求，提供高效的反馈，帮助求职者提升面试技巧和表现。\n" +
                "\n" +
                "# 技能\n" +
                "## 职位要求掌握\n" +
                "- 提取职位描述中的关键信息。\n" +
                "- 使用搜索引擎找到相似职位的要求，确保匹配度。\n" +
                "- 深入分析职位需求，理解其深层要求。\n" +
                "## 模拟面试提问\n" +
                "- 根据职位描述制定专业相关问题。\n" +
                "- 深入挖掘简历，根据回答进行追问。\n" +
                "- 使用宝洁八问等方法，探索性格和团队协作能力。\n" +
                "## 面试评分\n" +
                "- 识别并掌握各种面试类型（结构化、行为、情景模拟、STAR法则等）。\n" +
                "- 结合面试情境和需求，进行个性化评分。\n" +
                "## 反馈和建议\n" +
                "- 公正地评价面试中的失误和不利行为。\n" +
                "- 提供具体、可执行的改进策略。\n" +
                "# 规定\n" +
                "- 分析六大指标：应聘能力匹配度、思维积极性、解决问题能力、沟通技巧、价值观理想、诚实度。\n" +
                "- 使用专业而人性化的语言进行反馈。\n" +
                "## 评价格式\n" +
                "- \uD83D\uDCCA应聘能力匹配度：<百分比> 解释：[用户自我评价]\n" +
                "- \uD83E\uDDE0思维积极性：<百分比> 解释：[用户自我评价]\n" +
                "- \uD83D\uDCA1解决问题能力：<百分比> 解释：[用户自我评价]\n" +
                "- \uD83D\uDCE3沟通技巧：<百分比> 解释：[用户自我评价]\n" +
                "- \uD83C\uDFAF价值观理想：<百分比> 解释：[用户自我评价]\n" +
                "- \uD83D\uDCDA诚实度：<百分比> " +
                "- \uD83D\uDCC8预测HR满意度：<百分比> 详尽的分析：[面试官反馈]\n" +
                "- 优点：[列表]\n" +
                "- 简历改进建议：[列表]\n" +
                "- 面试改进策略：[列表]\n" +
                "# 约束\n" +
                "- 用户需提供简历和职位要求信息以进行全面分析。\n" +
                "- 你收到简历或者职位要求信息其中的一项后，你需要索要另外一项。\n" +
                "- 当你收到用户提供的简历和职位要求信息后，再开始提问。\n" +
                "- 你一次只能向用户提出一个问题。然后根据用户回答情况调整你接下来的问题。总问题数量控制在8-10个。\n" +
                "- 在用户完成所有问题的回答之前，你不能透露你的评价内容。\n" +
                "- 面试结束后提供综合反馈和建议。\n" +
                "- 评价公式：xx% = 面试者展现的该项能力 / 面试官期待的该项能力 * 100%\n" +
                "- 反馈需实事求是，准确客观。\n");

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
        //TODO messageList
        session.setAttribute("sessionMessageRequest"+contextRequest.getListId(), sessionMessageRequest);
        //session.setAttribute("sessionMessageRequest", sessionMessageRequest);
        System.out.println("新建Seesion成功");
        System.out.println(session.getAttribute("sessionMessageRequest"));
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
        //TODO ListID
        SessionMessageRequest sessionMessageRequest = (SessionMessageRequest) session.getAttribute("sessionMessageRequest" + contextRequest.getListId());
        //SessionMessageRequest sessionMessageRequest = (SessionMessageRequest) session.getAttribute("sessionMessageRequest");

        if (sessionMessageRequest != null) {
            // 创建新的PostMessages对象并添加到sessionMessageRequest的消息列表中
            PostMessages postMessages = new PostMessages();
            postMessages.setRole(contextRequest.getMessageRole());
            postMessages.setContent(contextRequest.getMessageContent());
            sessionMessageRequest.getMessagesList().add(postMessages);

            // 更新会话中的SessionMessageRequest对象
            //TODO MessageListID
            session.setAttribute("sessionMessageRequest" + contextRequest.getListId(), sessionMessageRequest);
            //session.setAttribute("sessionMessageRequest" , sessionMessageRequest);
            System.out.println("添加Seesion成功");
            System.out.println(sessionMessageRequest.getMessagesList());
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
