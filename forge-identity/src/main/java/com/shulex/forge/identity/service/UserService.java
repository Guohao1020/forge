package com.shulex.forge.identity.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.common.ErrorCode;
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
public class UserService {

    private final UserMapper userMapper;
    private final UserRoleMapper userRoleMapper;
    private final RoleMapper roleMapper;
    private final PasswordEncoder passwordEncoder;

    public UserService(UserMapper userMapper, UserRoleMapper userRoleMapper,
                       RoleMapper roleMapper, PasswordEncoder passwordEncoder) {
        this.userMapper = userMapper;
        this.userRoleMapper = userRoleMapper;
        this.roleMapper = roleMapper;
        this.passwordEncoder = passwordEncoder;
    }

    public UserDO createUser(Long tenantId, String username, String password, String nickname, String roleCode) {
        UserDO existing = userMapper.selectOne(new LambdaQueryWrapper<UserDO>()
                .eq(UserDO::getTenantId, tenantId)
                .eq(UserDO::getUsername, username));
        if (existing != null) {
            throw new BizException(ErrorCode.USER_EXISTS);
        }

        UserDO user = new UserDO();
        user.setTenantId(tenantId);
        user.setUsername(username);
        user.setPasswordHash(passwordEncoder.encode(password));
        user.setNickname(nickname);
        user.setStatus(1);
        userMapper.insert(user);

        if (roleCode != null) {
            RoleDO role = roleMapper.selectOne(new LambdaQueryWrapper<RoleDO>()
                    .eq(RoleDO::getTenantId, tenantId)
                    .eq(RoleDO::getRoleCode, roleCode));
            if (role != null) {
                UserRoleDO userRole = new UserRoleDO();
                userRole.setUserId(user.getId());
                userRole.setRoleId(role.getId());
                userRoleMapper.insert(userRole);
            }
        }

        log.info("创建用户: tenant={}, user={}, role={}", tenantId, username, roleCode);
        return user;
    }

    public UserDO getUserById(Long userId) {
        UserDO user = userMapper.selectById(userId);
        if (user == null) {
            throw new BizException(ErrorCode.USER_NOT_FOUND);
        }
        return user;
    }

    public List<UserDO> listUsers(Long tenantId) {
        return userMapper.selectList(new LambdaQueryWrapper<UserDO>()
                .eq(UserDO::getTenantId, tenantId)
                .orderByAsc(UserDO::getId));
    }
}
