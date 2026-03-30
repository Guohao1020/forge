package com.shulex.forge.identity.service;

import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.infrastructure.entity.ApiTokenDO;
import com.shulex.forge.identity.infrastructure.mapper.ApiTokenMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class ApiTokenServiceTest {

    private ApiTokenService apiTokenService;
    private ApiTokenMapper apiTokenMapper;

    @BeforeEach
    void setUp() {
        apiTokenMapper = mock(ApiTokenMapper.class);
        apiTokenService = new ApiTokenService(apiTokenMapper);
    }

    @Test
    void createToken_generatesForgePrefix() {
        when(apiTokenMapper.insert(any())).thenReturn(1);

        var result = apiTokenService.createToken(1L, 1L, "test-token", null);
        assertThat(result.getRawToken()).startsWith("forge_");
        assertThat(result.getTokenPrefix()).hasSize(12);
        assertThat(result.getTokenName()).isEqualTo("test-token");
        verify(apiTokenMapper).insert(any());
    }

    @Test
    void validateApiToken_returnsTokenOnMatch() {
        ApiTokenDO token = new ApiTokenDO();
        token.setId(1L);
        token.setStatus(1);
        when(apiTokenMapper.selectOne(any())).thenReturn(token);

        when(apiTokenMapper.insert(any())).thenReturn(1);
        var created = apiTokenService.createToken(1L, 1L, "test", null);

        ApiTokenDO result = apiTokenService.validateApiToken(created.getRawToken());
        assertThat(result).isNotNull();
    }

    @Test
    void validateApiToken_returnsNullOnNoMatch() {
        when(apiTokenMapper.selectOne(any())).thenReturn(null);
        assertThat(apiTokenService.validateApiToken("forge_invalid")).isNull();
    }

    @Test
    void revokeToken_setsStatusToZero() {
        ApiTokenDO token = new ApiTokenDO();
        token.setId(1L);
        token.setUserId(1L);
        token.setStatus(1);
        when(apiTokenMapper.selectById(1L)).thenReturn(token);
        when(apiTokenMapper.updateById(any())).thenReturn(1);

        apiTokenService.revokeToken(1L, 1L);
        assertThat(token.getStatus()).isEqualTo(0);
        verify(apiTokenMapper).updateById(token);
    }

    @Test
    void revokeToken_throwsWhenNotOwner() {
        ApiTokenDO token = new ApiTokenDO();
        token.setId(1L);
        token.setUserId(1L);
        token.setStatus(1);
        when(apiTokenMapper.selectById(1L)).thenReturn(token);

        assertThatThrownBy(() -> apiTokenService.revokeToken(1L, 999L))
                .isInstanceOf(BizException.class);
    }
}
