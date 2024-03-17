package com.yupi.springbootinit.controller;

import com.yupi.springbootinit.config.redis.RedisCacheUtils;
import com.yupi.springbootinit.config.redis.RedisUtil;
import com.yupi.springbootinit.model.dto.context.request.ContextRequest;

import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import java.util.ArrayList;
import java.util.List;

@RestController
@RequestMapping("/redis")
@Slf4j
public class RedisTestController {

    @Resource
    private RedisCacheUtils<ContextRequest> redisCache;

    @Autowired
    public RedisUtil rs;

    @GetMapping("/set")
    public Boolean testSet(@RequestParam String key, @RequestParam String value) {

        ContextRequest contextRequest = new ContextRequest();
        contextRequest.setUserAccount("kerikoTest");
        contextRequest.setMessageRole("user");
        contextRequest.setMessageContent(value);
        contextRequest.setListId(123456L);


        return redisCache.setObject(key, contextRequest);
    }


    @GetMapping("/get")
    @ResponseBody
    public List testGet(@RequestParam String key) {

            List listReis = rs.getList("wanli");
            if(listReis == null ){
                List list = new ArrayList();
                list.add(1);
                list.add(2);
                rs.setList("wanli",list);
            }
            return listReis;

    }
}
