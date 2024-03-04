package com.yupi.springbootinit.service;

import com.yupi.springbootinit.model.dto.context.ContextRequest;
import com.yupi.springbootinit.model.dto.moonshotai.PostReceiveRequest;
import com.yupi.springbootinit.model.dto.moonshotai.PostSendRequest;
import com.yupi.springbootinit.model.entity.Context;
import com.baomidou.mybatisplus.extension.service.IService;

import javax.servlet.http.HttpServletRequest;
import java.util.List;

/**
* @author Administrator
* @description 针对表【context】的数据库操作Service
* @createDate 2024-03-04 19:35:44
*/
public interface ContextService extends IService<Context> {
    /**
     * 回复用户请求
     * @param contextRequest 上下文请求对象，包含需要添加的上下文信息
     * @return 添加成功返回新增的上下文信息的ID，失败则返回-1
     */
    ContextRequest sendReceive(HttpServletRequest request, ContextRequest contextRequest);

    int addContextSql(ContextRequest contextRequest);
    int addReceiveSql(PostReceiveRequest postReceiveRequest, ContextRequest contextRequest);

    PostReceiveRequest sendMessagesToMoonshot(HttpServletRequest request, PostSendRequest postSendRequest);

    /**
     * 根据列表ID获取上下文信息列表
     * @param listId 列表的ID，用于指定需要查询的上下文信息列表
     * @return 返回对应列表ID的上下文信息列表，如果找不到则返回空列表
     */
    List<Context> listContext(Integer listId);

}
