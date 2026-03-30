package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.orchestration.model.KillSwitchLevel;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.*;

class KillSwitchServiceTest {

    private KillSwitchService killSwitchService;
    private StringRedisTemplate redisTemplate;
    private ValueOperations<String, String> valueOps;

    @BeforeEach
    void setUp() {
        redisTemplate = mock(StringRedisTemplate.class);
        valueOps = mock(ValueOperations.class);
        when(redisTemplate.opsForValue()).thenReturn(valueOps);
        killSwitchService = new KillSwitchService(redisTemplate);
    }

    @Test
    void getLevel_returnsNoneByDefault() {
        when(valueOps.get("forge:killswitch:level")).thenReturn(null);
        assertThat(killSwitchService.getLevel()).isEqualTo(KillSwitchLevel.NONE);
    }

    @Test
    void activate_setsLevel() {
        killSwitchService.activate(KillSwitchLevel.L1);
        verify(valueOps).set("forge:killswitch:level", "L1");
    }

    @Test
    void deactivate_setsNone() {
        killSwitchService.deactivate();
        verify(valueOps).set("forge:killswitch:level", "NONE");
    }

    @Test
    void isNewTaskAllowed_falseWhenL1() {
        when(valueOps.get("forge:killswitch:level")).thenReturn("L1");
        assertThat(killSwitchService.isNewTaskAllowed()).isFalse();
    }

    @Test
    void isExecutionAllowed_falseWhenL2() {
        when(valueOps.get("forge:killswitch:level")).thenReturn("L2");
        assertThat(killSwitchService.isExecutionAllowed()).isFalse();
    }
}
