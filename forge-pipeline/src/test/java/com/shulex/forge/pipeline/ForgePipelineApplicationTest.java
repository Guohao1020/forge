package com.shulex.forge.pipeline;

import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.test.context.ActiveProfiles;

@SpringBootTest
@ActiveProfiles("test")
class ForgePipelineApplicationTest {

    @MockBean
    StringRedisTemplate stringRedisTemplate;

    @Test
    void contextLoads() {
    }
}
