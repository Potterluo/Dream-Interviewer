package com.yupi.springbootinit.service;

import com.yupi.springbootinit.model.dto.context.ContextRequest;
import com.yupi.springbootinit.model.entity.Context;
import com.baomidou.mybatisplus.extension.service.IService;

import java.util.List;

/**
* @author Administrator
* @description 针对表【context】的数据库操作Service
* @createDate 2024-03-04 19:35:44
*/
public interface ContextService extends IService<Context> {
    int addContext(ContextRequest contextRequest);

    List<Context> listContext(Integer listId);

}
