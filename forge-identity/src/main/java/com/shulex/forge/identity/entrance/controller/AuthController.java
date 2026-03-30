package com.shulex.forge.identity.entrance.controller;

import com.shulex.forge.identity.common.Result;
import com.shulex.forge.identity.entrance.vo.LoginRequest;
import com.shulex.forge.identity.entrance.vo.LoginResponse;
import com.shulex.forge.identity.entrance.vo.RefreshRequest;
import com.shulex.forge.identity.service.AuthService;
import com.shulex.forge.identity.service.TokenService;
import io.jsonwebtoken.Claims;
import jakarta.validation.Valid;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

@RestController
@RequestMapping("/api/auth")
public class AuthController {

    private final AuthService authService;
    private final TokenService tokenService;

    public AuthController(AuthService authService, TokenService tokenService) {
        this.authService = authService;
        this.tokenService = tokenService;
    }

    @PostMapping("/login")
    public Result<LoginResponse> login(@Valid @RequestBody LoginRequest request) {
        LoginResponse response = authService.login(
                request.getTenantId(), request.getUsername(), request.getPassword());
        return Result.ok(response);
    }

    @PostMapping("/logout")
    public Result<Void> logout(@RequestHeader("Authorization") String authHeader) {
        if (!authHeader.startsWith("Bearer ")) {
            return Result.ok(null);
        }
        String token = authHeader.substring(7);
        authService.logout(token);
        return Result.ok(null);
    }

    @PostMapping("/refresh")
    public Result<LoginResponse> refresh(@Valid @RequestBody RefreshRequest request) {
        LoginResponse response = authService.refresh(request.getRefreshToken());
        return Result.ok(response);
    }

    @GetMapping("/verify")
    public ResponseEntity<Map<String, Object>> verify(
            @RequestHeader(value = "Authorization", required = false) String authHeader) {
        if (authHeader == null || !authHeader.startsWith("Bearer ")) {
            return ResponseEntity.status(401).body(Map.of("authenticated", false));
        }
        try {
            String token = authHeader.substring(7);
            Claims claims = tokenService.validateToken(token);
            return ResponseEntity.ok()
                    .header("X-User-Id", String.valueOf(claims.get("userId")))
                    .header("X-Tenant-Id", String.valueOf(claims.get("tenantId")))
                    .header("X-Username", claims.getSubject())
                    .body(Map.of(
                            "authenticated", true,
                            "userId", claims.get("userId"),
                            "tenantId", claims.get("tenantId"),
                            "username", claims.getSubject(),
                            "roles", claims.get("roles")
                    ));
        } catch (Exception e) {
            return ResponseEntity.status(401).body(Map.of("authenticated", false));
        }
    }
}
