package com.shulex.forge.identity.service;

import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.common.ErrorCode;
import com.shulex.forge.identity.infrastructure.entity.RoleDO;
import com.shulex.forge.identity.infrastructure.entity.UserDO;
import com.shulex.forge.identity.infrastructure.entity.UserRoleDO;
import com.shulex.forge.identity.infrastructure.mapper.RoleMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserRoleMapper;
import io.jsonwebtoken.impl.DefaultClaims;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.security.crypto.password.PasswordEncoder;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.*;

class AuthServiceTest {

    private AuthService authService;
    private UserMapper userMapper;
    private UserRoleMapper userRoleMapper;
    private RoleMapper roleMapper;
    private TokenService tokenService;
    private PasswordEncoder passwordEncoder;

    @BeforeEach
    void setUp() {
        userMapper = mock(UserMapper.class);
        userRoleMapper = mock(UserRoleMapper.class);
        roleMapper = mock(RoleMapper.class);
        tokenService = mock(TokenService.class);
        passwordEncoder = new BCryptPasswordEncoder();
        authService = new AuthService(userMapper, userRoleMapper, roleMapper, tokenService, passwordEncoder);
    }

    @Test
    void login_returnsTokensOnSuccess() {
        UserDO user = new UserDO();
        user.setId(1L);
        user.setTenantId(100L);
        user.setUsername("admin");
        user.setPasswordHash(passwordEncoder.encode("admin123"));
        user.setStatus(1);

        when(userMapper.selectOne(any())).thenReturn(user);

        UserRoleDO ur = new UserRoleDO();
        ur.setRoleId(1L);
        when(userRoleMapper.selectList(any())).thenReturn(List.of(ur));

        RoleDO role = new RoleDO();
        role.setRoleCode("ADMIN");
        when(roleMapper.selectById(1L)).thenReturn(role);

        when(tokenService.generateAccessToken(eq(1L), eq(100L), eq("admin"), any()))
                .thenReturn("access-token");
        when(tokenService.generateRefreshToken(1L, 100L, "admin"))
                .thenReturn("refresh-token");

        var result = authService.login(100L, "admin", "admin123");
        assertThat(result.getAccessToken()).isEqualTo("access-token");
        assertThat(result.getRefreshToken()).isEqualTo("refresh-token");
    }

    @Test
    void login_throwsOnWrongPassword() {
        UserDO user = new UserDO();
        user.setPasswordHash(passwordEncoder.encode("admin123"));
        user.setStatus(1);
        when(userMapper.selectOne(any())).thenReturn(user);

        assertThatThrownBy(() -> authService.login(100L, "admin", "wrong"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void login_throwsOnUserNotFound() {
        when(userMapper.selectOne(any())).thenReturn(null);

        assertThatThrownBy(() -> authService.login(100L, "nobody", "pass"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void login_throwsOnDisabledUser() {
        UserDO user = new UserDO();
        user.setStatus(0);
        when(userMapper.selectOne(any())).thenReturn(user);

        assertThatThrownBy(() -> authService.login(100L, "admin", "admin123"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void refresh_returnsNewTokensOnSuccess() {
        var claims = new DefaultClaims(Map.of(
                "type", "refresh", "userId", 1L, "tenantId", 100L, "sub", "admin"));
        when(tokenService.validateToken("refresh-token")).thenReturn(claims);

        UserDO user = new UserDO();
        user.setId(1L);
        user.setStatus(1);
        when(userMapper.selectById(1L)).thenReturn(user);
        when(userRoleMapper.selectList(any())).thenReturn(List.of());
        when(tokenService.generateAccessToken(eq(1L), eq(100L), eq("admin"), any()))
                .thenReturn("new-access");
        when(tokenService.generateRefreshToken(1L, 100L, "admin"))
                .thenReturn("new-refresh");

        var result = authService.refresh("refresh-token");
        assertThat(result.getAccessToken()).isEqualTo("new-access");
        assertThat(result.getRefreshToken()).isEqualTo("new-refresh");
        verify(tokenService).revokeToken("refresh-token");
    }

    @Test
    void refresh_throwsOnAccessToken() {
        var claims = new DefaultClaims(Map.of("type", "access"));
        when(tokenService.validateToken("access-token")).thenReturn(claims);

        assertThatThrownBy(() -> authService.refresh("access-token"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void refresh_throwsOnDisabledUser() {
        var claims = new DefaultClaims(Map.of(
                "type", "refresh", "userId", 1L, "tenantId", 100L, "sub", "admin"));
        when(tokenService.validateToken("refresh-token")).thenReturn(claims);

        UserDO user = new UserDO();
        user.setId(1L);
        user.setStatus(0);
        when(userMapper.selectById(1L)).thenReturn(user);

        assertThatThrownBy(() -> authService.refresh("refresh-token"))
                .isInstanceOf(BizException.class)
                .extracting(e -> ((BizException) e).getErrorCode())
                .isEqualTo(ErrorCode.USER_DISABLED);
    }

    @Test
    void refresh_throwsOnDeletedUser() {
        var claims = new DefaultClaims(Map.of(
                "type", "refresh", "userId", 999L, "tenantId", 100L, "sub", "deleted"));
        when(tokenService.validateToken("refresh-token")).thenReturn(claims);
        when(userMapper.selectById(999L)).thenReturn(null);

        assertThatThrownBy(() -> authService.refresh("refresh-token"))
                .isInstanceOf(BizException.class)
                .extracting(e -> ((BizException) e).getErrorCode())
                .isEqualTo(ErrorCode.USER_NOT_FOUND);
    }
}
