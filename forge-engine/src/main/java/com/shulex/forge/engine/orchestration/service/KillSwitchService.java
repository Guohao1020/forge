package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.orchestration.model.KillSwitchLevel;
import lombok.extern.slf4j.Slf4j;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class KillSwitchService {

    private static final String KEY = "forge:killswitch:level";
    private final StringRedisTemplate redisTemplate;

    public KillSwitchService(StringRedisTemplate redisTemplate) {
        this.redisTemplate = redisTemplate;
    }

    public KillSwitchLevel getLevel() {
        String val = redisTemplate.opsForValue().get(KEY);
        if (val == null) return KillSwitchLevel.NONE;
        try {
            return KillSwitchLevel.valueOf(val);
        } catch (IllegalArgumentException e) {
            return KillSwitchLevel.NONE;
        }
    }

    public void activate(KillSwitchLevel level) {
        redisTemplate.opsForValue().set(KEY, level.name());
        log.warn("紧急停止已激活: level={}", level);
    }

    public void deactivate() {
        redisTemplate.opsForValue().set(KEY, KillSwitchLevel.NONE.name());
        log.info("紧急停止已解除");
    }

    public boolean isNewTaskAllowed() {
        return getLevel() == KillSwitchLevel.NONE;
    }

    public boolean isExecutionAllowed() {
        KillSwitchLevel level = getLevel();
        return level == KillSwitchLevel.NONE;
    }
}
