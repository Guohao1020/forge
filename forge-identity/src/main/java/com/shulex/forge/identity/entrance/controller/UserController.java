package com.shulex.forge.identity.entrance.controller;

import com.shulex.forge.identity.common.Result;
import com.shulex.forge.identity.entrance.vo.CreateUserRequest;
import com.shulex.forge.identity.entrance.vo.UserVO;
import com.shulex.forge.identity.infrastructure.entity.UserDO;
import com.shulex.forge.identity.service.UserService;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.*;

import java.util.List;

@RestController
@RequestMapping("/api/users")
public class UserController {

    private final UserService userService;

    public UserController(UserService userService) {
        this.userService = userService;
    }

    @PostMapping
    public Result<UserVO> createUser(@Valid @RequestBody CreateUserRequest request) {
        UserDO user = userService.createUser(
                request.getTenantId(), request.getUsername(),
                request.getPassword(), request.getNickname(), request.getRoleCode());
        return Result.ok(toVO(user));
    }

    @GetMapping
    public Result<List<UserVO>> listUsers(@RequestParam(value = "tenantId") Long tenantId) {
        List<UserDO> users = userService.listUsers(tenantId);
        return Result.ok(users.stream().map(this::toVO).toList());
    }

    private UserVO toVO(UserDO user) {
        return UserVO.builder()
                .id(user.getId())
                .tenantId(user.getTenantId())
                .username(user.getUsername())
                .nickname(user.getNickname())
                .email(user.getEmail())
                .status(user.getStatus())
                .build();
    }
}
