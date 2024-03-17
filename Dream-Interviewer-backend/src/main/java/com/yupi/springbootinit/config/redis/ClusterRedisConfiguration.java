package com.yupi.springbootinit.config.redis;

import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.env.Environment;
import org.springframework.data.redis.connection.RedisStandaloneConfiguration;
import org.springframework.data.redis.connection.jedis.JedisClientConfiguration;
import org.springframework.data.redis.connection.jedis.JedisConnectionFactory;
import org.springframework.data.redis.core.RedisTemplate;
import redis.clients.jedis.JedisPoolConfig;

import javax.annotation.Resource;

/**
 * @author lixiaojin
 * @date 2021/6/11 22:29
 */
@Configuration
public class ClusterRedisConfiguration {

    @Resource
    private Environment env;

    @Bean
    public JedisConnectionFactory redisConnectionFactory() {
        JedisPoolConfig jedisPoolConfig = new JedisPoolConfig();
        jedisPoolConfig.setMaxTotal(200);
        jedisPoolConfig.setMaxIdle(150);
        jedisPoolConfig.setMinIdle(50);

        RedisStandaloneConfiguration redisStandaloneConfiguration = new RedisStandaloneConfiguration();
        redisStandaloneConfiguration.setHostName(env.getProperty("spring.redis.host"));
        redisStandaloneConfiguration.setPort(Integer.parseInt(env.getProperty("spring.redis.port")));
        redisStandaloneConfiguration.setPassword(env.getProperty("spring.redis.password"));
        redisStandaloneConfiguration.setDatabase(Integer.parseInt(env.getProperty("spring.redis.database")));

        JedisClientConfiguration.JedisClientConfigurationBuilder configurationBuilder = JedisClientConfiguration.builder();
        JedisClientConfiguration jedisClientConfiguration = configurationBuilder.usePooling().poolConfig(jedisPoolConfig).build();

        return new JedisConnectionFactory(redisStandaloneConfiguration, jedisClientConfiguration);
    }

    /**
     * 缓存模板--对象序列化
     */
    @Bean
    @ConditionalOnMissingBean(name = "redisTemplate")
    public RedisTemplate<String, Object> redisTemplate(JedisConnectionFactory jedisConnectionFactory) {
        RedisTemplate<String, Object> template = new RedisTemplate<>();
        //使用fastJson序列化
        FastJsonRedisSerializer<Object> fastJsonRedisSerializer = new FastJsonRedisSerializer<>(Object.class);
        //value值序列化
        template.setValueSerializer(fastJsonRedisSerializer);
        template.setHashValueSerializer(fastJsonRedisSerializer);
        //key值序列化
        template.setKeySerializer(fastJsonRedisSerializer);
        template.setHashKeySerializer(fastJsonRedisSerializer);
        //设置连接
        template.setConnectionFactory(jedisConnectionFactory);
        return template;
    }

}