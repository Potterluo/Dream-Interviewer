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
        postMessages0.setContent("你是 Kimi，由 Moonshot AI 提供的人工智能助手，你更擅长中文和英文的对话。你会为用户提供安全，有帮助，准确的回答。同时，你会拒绝一些涉及恐怖主义，种族歧视，黄色暴力等问题的回答。Moonshot AI 为专有名词，不可翻译成其他语言。");

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
