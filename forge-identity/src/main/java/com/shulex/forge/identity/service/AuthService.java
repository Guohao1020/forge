package com.shulex.forge.identity.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.common.ErrorCode;
import com.shulex.forge.identity.entrance.vo.LoginResponse;
import com.shulex.forge.identity.infrastructure.entity.RoleDO;
import com.shulex.forge.identity.infrastructure.entity.UserDO;
import com.shulex.forge.identity.infrastructure.entity.UserRoleDO;
import com.shulex.forge.identity.infrastructure.mapper.RoleMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserRoleMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;

import java.util.List;

@Slf4j
@Service
public class AuthService {

    private final UserMapper userMapper;
    private final UserRoleMapper userRoleMapper;
    private final RoleMapper roleMapper;
    private final TokenService tokenService;
    private final PasswordEncoder passwordEncoder;

    public AuthService(UserMapper userMapper, UserRoleMapper userRoleMapper,
                       RoleMapper roleMapper, TokenService tokenService,
                       PasswordEncoder passwordEncoder) {
        this.userMapper = userMapper;
        this.userRoleMapper = userRoleMapper;
        this.roleMapper = roleMapper;
        this.tokenService = tokenService;
        this.passwordEncoder = passwordEncoder;
    }

    public LoginResponse login(Long tenantId, String username, String password) {
        UserDO user = userMapper.selectOne(new LambdaQueryWrapper<UserDO>()
                .eq(UserDO::getTenantId, tenantId)
                .eq(UserDO::getUsername, username));
        if (user == null) {
            throw new BizException(ErrorCode.INVALID_CREDENTIALS);
        }
        if (user.getStatus() == 0) {
            throw new BizException(ErrorCode.USER_DISABLED);
        }
        if (!passwordEncoder.matches(password, user.getPasswordHash())) {
            throw new BizException(ErrorCode.INVALID_CREDENTIALS);
        }

        List<String> roles = getUserRoles(user.getId());
        String accessToken = tokenService.generateAccessToken(user.getId(), tenantId, username, roles);
        String refreshToken = tokenService.generateRefreshToken(user.getId(), tenantId, username);

        log.info("用户登录成功: tenant={}, user={}", tenantId, username);
        return LoginResponse.builder()
                .accessToken(accessToken)
                .refreshToken(refreshToken)
                .userId(user.getId())
                .username(username)
                .roles(roles)
                .build();
    }

    public LoginResponse refresh(String refreshToken) {
        var claims = tokenService.validateToken(refreshToken);
        if (!"refresh".equals(claims.get("type", String.class))) {
            throw new BizException(ErrorCode.TOKEN_INVALID, "非 Refresh Token");
        }
        Long userId = claims.get("userId", Long.class);
        Long tenantId = claims.get("tenantId", Long.class);
        String username = claims.getSubject();

        UserDO user = userMapper.selectById(userId);
        if (user == null) {
            throw new BizException(ErrorCode.USER_NOT_FOUND);
        }
        if (user.getStatus() == 0) {
            throw new BizException(ErrorCode.USER_DISABLED);
        }

        List<String> roles = getUserRoles(userId);
        String newAccessToken = tokenService.generateAccessToken(userId, tenantId, username, roles);
        String newRefreshToken = tokenService.generateRefreshToken(userId, tenantId, username);

        tokenService.revokeToken(refreshToken);

        return LoginResponse.builder()
                .accessToken(newAccessToken)
                .refreshToken(newRefreshToken)
                .userId(userId)
                .username(username)
                .roles(roles)
                .build();
    }

    public void logout(String accessToken) {
        tokenService.revokeToken(accessToken);
    }

    private List<String> getUserRoles(Long userId) {
        List<UserRoleDO> userRoles = userRoleMapper.selectList(
                new LambdaQueryWrapper<UserRoleDO>().eq(UserRoleDO::getUserId, userId));
        if (userRoles.isEmpty()) {
            return List.of();
        }
        List<Long> roleIds = userRoles.stream().map(UserRoleDO::getRoleId).toList();
        List<RoleDO> roles = roleMapper.selectList(
                new LambdaQueryWrapper<RoleDO>().in(RoleDO::getId, roleIds));
        return roles.stream().map(RoleDO::getRoleCode).toList();
    }
}
