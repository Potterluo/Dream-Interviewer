package com.yupi.springbootinit.service;


import com.baomidou.mybatisplus.extension.service.IService;
import com.yupi.springbootinit.model.entity.MessageList;

import java.util.List;

/**
* @author Administrator
* @description 针对表【message_list】的数据库操作Service
* @createDate 2024-03-03 20:40:00
*/
public interface MessageListService extends IService<MessageList> {
    /*int AddMessageList(String userAccount);*/
    int addMessageList(String userAccount);
    boolean deleteMessageList(Integer listId);

    String mainMessage(Integer ListId);

    List<MessageList> listMessageList(String userAccount);

}



