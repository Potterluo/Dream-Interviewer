package com.yupi.springbootinit.controller;

import com.yupi.springbootinit.model.dto.context.ContextRequest;
import com.yupi.springbootinit.model.entity.Context;
import com.yupi.springbootinit.service.ContextService;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import javax.annotation.Resource;
import java.util.List;

@RestController
@RequestMapping("/context")
@Slf4j
public class ContextController {
    @Resource
    private ContextService contextService;

    @PostMapping("/add")
    public int addContext(@RequestBody ContextRequest contextRequest) {
        return contextService.addContext(contextRequest);
    }

    @PostMapping("/list")
    public List<Context> listContext(Integer listId) {
        return contextService.listContext(listId);
    }

}
