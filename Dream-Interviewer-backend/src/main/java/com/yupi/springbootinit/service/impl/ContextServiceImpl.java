package com.yupi.springbootinit.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import com.yupi.springbootinit.common.ErrorCode;
import com.yupi.springbootinit.exception.BusinessException;
import com.yupi.springbootinit.model.dto.context.ContextRequest;
import com.yupi.springbootinit.model.entity.Context;
import com.yupi.springbootinit.service.ContextService;
import com.yupi.springbootinit.mapper.ContextMapper;
import org.springframework.stereotype.Service;

import java.util.List;

/**
* @author Administrator
* @description 针对表【context】的数据库操作Service实现
* @createDate 2024-03-04 19:35:44
*/
@Service
public class ContextServiceImpl extends ServiceImpl<ContextMapper, Context>
    implements ContextService{
    public int addContext(ContextRequest contextRequest){
        Context context = new Context();
        context.setTimeStamp(new java.util.Date());
        context.setUserAccount(contextRequest.getUserAccount());
        context.setMessageRole(contextRequest.getMessageRole());
        //byte[] bytes = messageContent.getBytes(StandardCharsets.UTF_8);
        context.setMessageContent(contextRequest.getMessageContent());
        context.setListId(contextRequest.getListId());
        context.setMessageModel("moonshot-v1-8k");
        boolean saveResult = this.save(context);
        if (!saveResult) {
            throw new BusinessException(ErrorCode.SYSTEM_ERROR, "发送消息失败");
        }
        return context.getId();
    }
    public List<Context> listContext(Integer listId){
        return this.list(new QueryWrapper<Context>().eq("listId", listId));
    }

}




