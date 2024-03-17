package com.yupi.springbootinit.config.redis;

import org.apache.commons.lang3.StringUtils;
import org.springframework.data.redis.core.RedisTemplate;
import org.springframework.stereotype.Component;

import javax.annotation.Resource;
import java.util.concurrent.TimeUnit;



/**
 * @param <T>
 * @author lixiaojin
 * @date 2021-06-11
 */
@Component
public class RedisCacheUtils<T> {

    @Resource
    private RedisTemplate<String, T> redisTemplate;

    /**
     * 获取存储的对象
     *
     * @param key key
     * @return 对象
     */
    public T getObject(String key) {
        if (StringUtils.isEmpty(key)) {
            return null;
        }
        return redisTemplate.opsForValue().get(key);
    }

    /**
     * 存入对象类型数据
     *
     * @param key   redis key
     * @param value redis value
     */
    public boolean setObject(String key, T value) {
        return setObject(key, value, 0);
    }

    /**
     * 存入对象类型数据，设置存储时间
     *
     * @param key   key
     * @param value value
     * @param time  存放时间
     */
    public boolean setObject(String key, T value, long time) {
        if (StringUtils.isEmpty(key) || value == null) {
            return false;
        }
        if (time == 0) {
            redisTemplate.opsForValue().set(key, value);
        } else {
            redisTemplate.opsForValue().set(key, value, time, TimeUnit.SECONDS);
        }
        return true;
    }

    /**
     * 重新设置对象失效时间
     *
     * @param key
     * @param time
     * @return
     */
    public boolean addObjInvalidTime(String key, long time) {
        if (StringUtils.isEmpty(key) && redisTemplate.getExpire(key) <= 0) {
            return false;
        }
        return redisTemplate.expire(key, time, TimeUnit.SECONDS);

    }

    /**
     * 根据key 清除缓存
     *
     * @param key key
     * @return 清楚结果
     */
    public boolean removeObj(String key) {
        if (StringUtils.isEmpty(key)) {
            return false;
        }
        redisTemplate.delete(key);
        return true;
    }

}