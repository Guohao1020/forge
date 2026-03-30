package com.shulex.forge.identity.entrance.controller;

import com.shulex.forge.identity.common.Result;
import com.shulex.forge.identity.entrance.vo.ApiTokenVO;
import com.shulex.forge.identity.service.ApiTokenService;
import io.jsonwebtoken.Claims;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

import java.time.LocalDateTime;
import java.util.List;

@RestController
@RequestMapping("/api/tokens")
public class ApiTokenController {

    private final ApiTokenService apiTokenService;

    public ApiTokenController(ApiTokenService apiTokenService) {
        this.apiTokenService = apiTokenService;
    }

    @PostMapping
    public Result<ApiTokenVO> createToken(
            @RequestParam(value = "name") String name,
            @RequestParam(value = "expireDays", required = false) Integer expireDays,
            Authentication authentication) {
        Claims claims = (Claims) authentication.getDetails();
        Long userId = claims.get("userId", Long.class);
        Long tenantId = claims.get("tenantId", Long.class);
        LocalDateTime expiresAt = expireDays != null
                ? LocalDateTime.now().plusDays(expireDays) : null;
        return Result.ok(apiTokenService.createToken(tenantId, userId, name, expiresAt));
    }

    @GetMapping
    public Result<List<ApiTokenVO>> listTokens(Authentication authentication) {
        Claims claims = (Claims) authentication.getDetails();
        Long userId = claims.get("userId", Long.class);
        Long tenantId = claims.get("tenantId", Long.class);
        return Result.ok(apiTokenService.listTokens(tenantId, userId));
    }

    @DeleteMapping("/{tokenId}")
    public Result<Void> revokeToken(@PathVariable("tokenId") Long tokenId) {
        apiTokenService.revokeToken(tokenId);
        return Result.ok(null);
    }
}
