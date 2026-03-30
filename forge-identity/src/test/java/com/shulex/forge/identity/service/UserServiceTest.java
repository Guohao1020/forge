package com.shulex.forge.identity.service;

import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.infrastructure.entity.RoleDO;
import com.shulex.forge.identity.infrastructure.entity.UserDO;
import com.shulex.forge.identity.infrastructure.entity.UserRoleDO;
import com.shulex.forge.identity.infrastructure.mapper.RoleMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserMapper;
import com.shulex.forge.identity.infrastructure.mapper.UserRoleMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class UserServiceTest {

    private UserService userService;
    private UserMapper userMapper;
    private UserRoleMapper userRoleMapper;
    private RoleMapper roleMapper;

    @BeforeEach
    void setUp() {
        userMapper = mock(UserMapper.class);
        userRoleMapper = mock(UserRoleMapper.class);
        roleMapper = mock(RoleMapper.class);
        userService = new UserService(userMapper, userRoleMapper, roleMapper, new BCryptPasswordEncoder());
    }

    @Test
    void createUser_insertsUserAndAssignsRole() {
        when(userMapper.selectOne(any())).thenReturn(null);
        when(userMapper.insert(any())).thenReturn(1);

        RoleDO role = new RoleDO();
        role.setId(2L);
        when(roleMapper.selectOne(any())).thenReturn(role);
        when(userRoleMapper.insert(any())).thenReturn(1);

        UserDO created = userService.createUser(100L, "newuser", "password123", "New User", "ADMIN");
        assertThat(created.getUsername()).isEqualTo("newuser");
        verify(userMapper).insert(any());
        verify(userRoleMapper).insert(any());
    }

    @Test
    void createUser_throwsOnDuplicate() {
        when(userMapper.selectOne(any())).thenReturn(new UserDO());

        assertThatThrownBy(() -> userService.createUser(100L, "existing", "pass", "name", "USER"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void getUserById_returnsUser() {
        UserDO user = new UserDO();
        user.setId(1L);
        user.setUsername("admin");
        when(userMapper.selectById(1L)).thenReturn(user);

        UserDO result = userService.getUserById(1L);
        assertThat(result.getUsername()).isEqualTo("admin");
    }

    @Test
    void getUserById_throwsOnNotFound() {
        when(userMapper.selectById(999L)).thenReturn(null);
        assertThatThrownBy(() -> userService.getUserById(999L))
                .isInstanceOf(BizException.class);
    }
}
