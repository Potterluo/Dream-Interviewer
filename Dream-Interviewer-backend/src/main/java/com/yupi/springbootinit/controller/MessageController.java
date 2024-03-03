package com.yupi.springbootinit.controller;

import com.yupi.springbootinit.common.BaseResponse;
import com.yupi.springbootinit.model.entity.MessageList;
import com.yupi.springbootinit.service.MessageListService;
import com.yupi.springbootinit.service.UserService;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import javax.annotation.Resource;
import java.util.List;

@RestController
@RequestMapping("/message")
@Slf4j
public class MessageController {

    @Resource
    private MessageListService messageListservice;

    /**
     * 添加消息列表。
     *
     * @param userAccount 用户账号，用于标识消息列表的归属。
     * @return 返回添加操作的结果，通常为成功或失败的状态码。
     */
    @PostMapping("/add")
    public int addMessageList(String userAccount)
    {
        return  messageListservice.addMessageList(userAccount);
    }

    /**
     * 删除消息列表。
     *
     * @param listId 消息列表的ID，用于唯一标识要删除的消息列表。
     * @return 返回删除操作的结果，通常为成功或失败的布尔值。
     */
    @GetMapping("/delete")
    public boolean deleteMessageList(int listId)
    {
        return  messageListservice.deleteMessageList(listId);
    }

/**
 * 获取主消息。
 *
 * @param  listId 消息列表对象，其中可能包含用于筛选主消息的相关信息。
 * @return 返回获取到的主消息内容或处理结果。
 */
@GetMapping("/main")
public String mainMessage(int listId)
{
    return  messageListservice.mainMessage(listId);
}

/**
 * 列出用户的消息列表。
 *
 * @param userAccount 用户账号，用于标识要列出消息列表的归属用户。
 * @return 返回指定用户的消息列表集合。
 */
@GetMapping("/list")
public List<MessageList> listMessageList(String userAccount)
{
    return  messageListservice.listMessageList(userAccount);
}

}