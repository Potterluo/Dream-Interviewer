package com.yupi.springbootinit.controller;

import com.yupi.springbootinit.model.dto.context.request.ContextRequest;
import com.yupi.springbootinit.model.entity.Context;
import com.yupi.springbootinit.service.ContextService;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletRequest;
import java.util.List;

/**
 * 上下文控制器
 * 用于处理与上下文相关的RESTful请求
 */
@RestController
@RequestMapping("/context")
@Slf4j
public class ContextController {
    // 注入上下文服务
    @Resource
    private ContextService contextService;

    /**
     * 添加上下文信息
     * @param contextRequest 上下文请求对象，包含要添加的上下文信息
     * @return 添加成功的上下文数量
     */
    @PostMapping("/add")
    public ContextRequest addContext(@RequestBody ContextRequest contextRequest, HttpServletRequest request) {
        return contextService.sendReceive(request,contextRequest);
    }

 /*   @PostMapping("/chat")
    public PostReceiveRequest chat(@RequestBody PostSendRequest postSendRequest, HttpServletRequest request) {
        return contextService.sendMessagesToMoonshot(request,postSendRequest);
    }*/

    @PostMapping("/chat")
    public ContextRequest chat(@RequestBody ContextRequest contextRequest, HttpServletRequest request) {
        return contextService.sendReceive(request,contextRequest);
    }

    /**
     * 查询上下文列表
     * @param listId 列表ID，可选参数，用于查询特定ID的上下文列表
     * @return 上下文列表
     */
    @PostMapping("/list")
    public List<Context> listContext(Integer listId) {
        return contextService.listContext(listId);
    }

}

