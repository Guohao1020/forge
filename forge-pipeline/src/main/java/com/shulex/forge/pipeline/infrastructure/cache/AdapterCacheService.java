package com.shulex.forge.pipeline.infrastructure.cache;

import lombok.extern.slf4j.Slf4j;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.util.Optional;

@Slf4j
@Service
public class AdapterCacheService {

    private final StringRedisTemplate redisTemplate;
    private static final String KEY_PREFIX = "forge:adapter:cache:";

    public AdapterCacheService(StringRedisTemplate redisTemplate) {
        this.redisTemplate = redisTemplate;
    }

    public Optional<String> get(String key) {
        String value = redisTemplate.opsForValue().get(KEY_PREFIX + key);
        if (value != null) {
            log.debug("缓存命中: {}", key);
        }
        return Optional.ofNullable(value);
    }

    public void put(String key, String value, Duration ttl) {
        redisTemplate.opsForValue().set(KEY_PREFIX + key, value, ttl);
        log.debug("缓存写入: {}, TTL: {}", key, ttl);
    }

    public void evict(String key) {
        redisTemplate.delete(KEY_PREFIX + key);
    }

    public void evictByPrefix(String prefix) {
        var keys = redisTemplate.keys(KEY_PREFIX + prefix + "*");
        if (keys != null && !keys.isEmpty()) {
            redisTemplate.delete(keys);
            log.debug("缓存批量清除: {} 个 key", keys.size());
        }
    }
}
