package com.shulex.forge.identity.service;

import com.shulex.forge.identity.common.BizException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

class TokenServiceTest {

    private TokenService tokenService;
    private StringRedisTemplate redisTemplate;
    private ValueOperations<String, String> valueOps;

    @BeforeEach
    void setUp() {
        redisTemplate = mock(StringRedisTemplate.class);
        valueOps = mock(ValueOperations.class);
        when(redisTemplate.opsForValue()).thenReturn(valueOps);
        tokenService = new TokenService(
                "test-secret-key-for-unit-tests-must-be-at-least-256-bits-long-enough",
                30, 10080, redisTemplate);
    }

    @Test
    void generateAccessToken_containsExpectedClaims() {
        String token = tokenService.generateAccessToken(1L, 100L, "admin", List.of("ADMIN"));
        var claims = tokenService.parseToken(token);
        assertThat(claims.get("userId", Long.class)).isEqualTo(1L);
        assertThat(claims.get("tenantId", Long.class)).isEqualTo(100L);
        assertThat(claims.getSubject()).isEqualTo("admin");
    }

    @Test
    void generateRefreshToken_isValid() {
        String token = tokenService.generateRefreshToken(1L, 100L, "admin");
        var claims = tokenService.parseToken(token);
        assertThat(claims.get("type", String.class)).isEqualTo("refresh");
    }

    @Test
    void revokeToken_addsToBlacklist() {
        String token = tokenService.generateAccessToken(1L, 100L, "admin", List.of("ADMIN"));
        tokenService.revokeToken(token);
        verify(valueOps).set(startsWith("forge:token:blacklist:"), eq("revoked"), any());
    }

    @Test
    void validateToken_throwsWhenRevoked() {
        when(redisTemplate.hasKey(anyString())).thenReturn(true);
        String token = tokenService.generateAccessToken(1L, 100L, "admin", List.of("ADMIN"));
        assertThatThrownBy(() -> tokenService.validateToken(token))
                .isInstanceOf(BizException.class);
    }

    @Test
    void parseToken_throwsOnInvalidToken() {
        assertThatThrownBy(() -> tokenService.parseToken("invalid.token.here"))
                .isInstanceOf(BizException.class);
    }
}
