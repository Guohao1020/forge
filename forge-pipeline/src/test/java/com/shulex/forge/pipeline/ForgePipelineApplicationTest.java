package com.shulex.forge.pipeline;

import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.test.context.ActiveProfiles;

@SpringBootTest
@ActiveProfiles("test")
class ForgePipelineApplicationTest {

    @MockBean
    private StringRedisTemplate redisTemplate;

    @MockBean
    private AdapterRegistry adapterRegistry;

    @Test
    void contextLoads() {
    }
}
