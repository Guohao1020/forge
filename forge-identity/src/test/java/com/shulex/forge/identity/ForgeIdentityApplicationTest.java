package com.shulex.forge.identity;

import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.test.context.ActiveProfiles;

@SpringBootTest
@ActiveProfiles("test")
class ForgeIdentityApplicationTest {

    @MockBean
    private StringRedisTemplate redisTemplate;

    @Test
    void contextLoads() {
    }
}
