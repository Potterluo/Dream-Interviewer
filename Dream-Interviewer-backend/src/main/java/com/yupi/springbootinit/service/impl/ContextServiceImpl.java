package com.yupi.springbootinit.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.yupi.springbootinit.common.ErrorCode;
import com.yupi.springbootinit.exception.BusinessException;
import com.yupi.springbootinit.model.dto.context.request.ContextRequest;
import com.yupi.springbootinit.model.dto.context.request.SessionMessageRequest;
import com.yupi.springbootinit.model.dto.moonshotai.PostMessages;
import com.yupi.springbootinit.model.dto.moonshotai.PostReceiveChoices;
import com.yupi.springbootinit.model.dto.moonshotai.PostReceiveRequest;
import com.yupi.springbootinit.model.dto.moonshotai.PostSendRequest;
import com.yupi.springbootinit.model.entity.Context;
import com.yupi.springbootinit.service.ContextService;
import com.yupi.springbootinit.mapper.ContextMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.*;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestTemplate;

import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpSession;
import java.util.List;

/**
* @author Administrator
* @description 针对表【context】的数据库操作Service实现
* @createDate 2024-03-04 19:35:44
*/
@Service
public class ContextServiceImpl extends ServiceImpl<ContextMapper, Context>
    implements ContextService{
    public ContextRequest sendReceive(HttpServletRequest request, ContextRequest contextRequest){

        //存数据库*/
        int contextId = addContextSql(contextRequest);
        //存Session
        SessionMessageService sessionMessageService = new SessionMessageService();
        sessionMessageService.addMessageList(request,contextRequest);
        //读session
        long listId = contextRequest.getListId();
        HttpSession session = request.getSession();
        //TODO 文件传输接口不能传递回来ListId信息，暂时不做ListId区分
        SessionMessageRequest  contextSession = (SessionMessageRequest) session.getAttribute("sessionMessageRequest"+contextRequest.getListId());
        //SessionMessageRequest  contextSession = (SessionMessageRequest) session.getAttribute("sessionMessageRequest");
        //获取发送对象
        PostSendRequest postSendRequest = contextSession.getPostSendRequest();
        //请MoonShot发出调用请求
        PostReceiveRequest postReceiveRequest = sendMessagesToMoonshot(request,postSendRequest);
        //存数据库
        addReceiveSql(postReceiveRequest,contextRequest);
        //获取返回的receiveContextRequest
        ContextRequest receiveContextRequest = new ContextRequest();
        receiveContextRequest.setListId(listId);
        receiveContextRequest.setUserAccount(contextRequest.getUserAccount());
        PostReceiveChoices postReceiveChoices = postReceiveRequest.getChoices()[0];
        receiveContextRequest.setMessageRole(postReceiveChoices.getMessage().getRole());
        receiveContextRequest.setMessageContent(postReceiveChoices.getMessage().getContent());
        //存session
        sessionMessageService.addMessageList(request,receiveContextRequest);

        return receiveContextRequest;
    }
    /**
     * 把用户发送的对话信息存入数据库中
     * @param contextRequest 上下文请求对象，包含用户账号、消息角色、消息内容和列表ID等信息
     * @return 添加成功返回上下文的ID
     * @throws BusinessException 如果保存上下文失败，则抛出业务异常
     */
    public int addContextSql(ContextRequest contextRequest){
        // 创建上下文对象并设置时间戳、用户账号、消息角色、消息内容和列表ID
        Context context = new Context();
        context.setTimeStamp(new java.util.Date());
        context.setUserAccount(contextRequest.getUserAccount());
        context.setMessageRole(contextRequest.getMessageRole());
        context.setMessageContent(contextRequest.getMessageContent());
        context.setListId(contextRequest.getListId());
        context.setMessageModel("moonshot-v1-8k");

        // 保存上下文信息到数据库
        boolean saveResult = this.save(context);
        // 如果保存失败，抛出业务异常
        if (!saveResult) {
            throw new BusinessException(ErrorCode.SYSTEM_ERROR, "发送消息失败");
        }

        // 返回上下文ID
        return context.getId();
    }

    /**
     * 将接收的返回信息添加到上下文中并保存至数据库
     *
     * @param postReceiveRequest 包含接收选项的请求对象
     * @param contextRequest 包含上下文信息的请求对象，如用户账号和列表ID
     * @return 保存成功后返回的对话ID
     * @throws BusinessException 如果保存失败，则抛出业务异常
     */
    public int addReceiveSql(PostReceiveRequest postReceiveRequest, ContextRequest contextRequest){
        // 初始化上下文对象并设置时间戳、用户账号等信息
        Context context = new Context();
        context.setTimeStamp(new java.util.Date());
        context.setUserAccount(contextRequest.getUserAccount());

        // 解析请求中的消息角色和内容，并设置到上下文中
        PostReceiveChoices postReceiveChoices = postReceiveRequest.getChoices()[0];
        PostMessages postMessages = postReceiveChoices.getMessage();
        context.setMessageRole(postMessages.getRole());
        context.setMessageContent(postMessages.getContent());

        // 设置列表ID和消息模型
        context.setListId(contextRequest.getListId());
        context.setMessageModel("moonshot-v1-8k");

        // 保存上下文信息到数据库
        boolean saveResult = this.save(context);
        // 如果保存失败，抛出业务异常
        if (!saveResult) {
            throw new BusinessException(ErrorCode.SYSTEM_ERROR, "发送消息失败");
        }

        // 返回保存后的上下文ID
        return context.getId();
    }

/**
 * 向Moonshot AI发送消息并接收响应。
 *
 * @param request HttpServletRequest对象，用于接收客户端请求（此示例中未使用）。
 * @param postSendRequest 包含要发送到Moonshot AI的消息内容的PostSendRequest对象。
 * @return PostReceiveRequest对象，包含从Moonshot AI接收到的响应。
 */
//@Value("${moonshot.api.key}")
String API_KEY = "sk-EuMYqAL9j2h3aa1Pk8jqN0b49bdqJGp1JZLR1Cpn2faICAdh" ;
//@Value("${moonshot.api.base-url}")
String API_BASE_URL = "https://api.moonshot.cn/v1";
public PostReceiveRequest sendMessagesToMoonshot(HttpServletRequest request, PostSendRequest postSendRequest){

    // 设置API密钥

    RestTemplate restTemplate = new RestTemplate();
    HttpHeaders headers = new HttpHeaders();
    headers.setContentType(MediaType.APPLICATION_JSON);
    // 在请求头中添加Authorization信息
    headers.set("Authorization", "Bearer " + API_KEY);
    HttpEntity<Object> entity = new HttpEntity<>(postSendRequest, headers);

    try {
        // 使用RestTemplate发送POST请求
        ResponseEntity<String> response = restTemplate.exchange(
                /*API_BASE_URL + "/chat/completions"*/
                "https://api.moonshot.cn/v1/chat/completions",
                HttpMethod.POST,
                entity,
                String.class);
        ObjectMapper mapper = new ObjectMapper();
        // 将API响应转换为PostReceiveRequest对象
        return mapper.readValue(response.getBody(), PostReceiveRequest.class);
    } catch (Exception e) {
        // 处理请求过程中发生的异常
        throw new RuntimeException("访问服务器失败");
    }
}


    public List<Context> listContext(Integer listId){
        return this.list(new QueryWrapper<Context>().eq("listId", listId));
    }

}




