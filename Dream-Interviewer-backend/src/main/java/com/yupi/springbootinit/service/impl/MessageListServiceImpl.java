package com.yupi.springbootinit.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import com.yupi.springbootinit.common.ErrorCode;
import com.yupi.springbootinit.exception.BusinessException;
import com.yupi.springbootinit.model.entity.MessageList;

import com.yupi.springbootinit.mapper.MessageListMapper;
import com.yupi.springbootinit.service.MessageListService;
import org.springframework.stereotype.Service;

import java.util.List;

/**
* @author Administrator
* @description 针对表【message_list】的数据库操作Service实现
* @createDate 2024-03-03 20:40:00
*/
@Service
public class MessageListServiceImpl extends ServiceImpl<MessageListMapper, MessageList>
    implements MessageListService {


/*    @Override
    int AddMessageList(String userAccount){
        MessageList messageList = new MessageList();
        messageList.setUserAccount(userAccount);
        messageList.setCreateTime(new java.util.Date());
        messageList.setIsDelete(0);
        return 0;
    }*/
    @Override
    public int addMessageList(String userAccount){
        MessageList messageList = new MessageList();
        messageList.setUserAccount(userAccount);
        messageList.setCreateTime(new java.util.Date());
        messageList.setIsDelete(0);
        boolean saveResult = this.save(messageList);
        if (!saveResult) {
            throw new BusinessException(ErrorCode.SYSTEM_ERROR, "新建会话失败");
        }
        return messageList.getListId();
    }

    @Override
    public boolean deleteMessageList(Integer listId){
        boolean removeResult = this.removeById(listId);
        if (!removeResult) {
            throw new BusinessException(ErrorCode.SYSTEM_ERROR, "删除会话失败");
        }
        return removeResult;
    }

    @Override
    public String mainMessage(Integer listId){
        MessageList messageList = this.getById(listId);
        String mainContent = messageList.getMainContent();
/*        if(!mainContent.equals("")){
            throw new BusinessException(ErrorCode.SYSTEM_ERROR, "为获取到主要内容");
        }*/
        return mainContent;
    }

    @Override
    public List<MessageList> listMessageList(String userAccount){
        return this.list(new QueryWrapper<MessageList>().eq("userAccount", userAccount));
    }





}




