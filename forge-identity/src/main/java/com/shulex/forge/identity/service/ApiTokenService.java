package com.shulex.forge.identity.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.shulex.forge.identity.entrance.vo.ApiTokenVO;
import com.shulex.forge.identity.infrastructure.entity.ApiTokenDO;
import com.shulex.forge.identity.infrastructure.mapper.ApiTokenMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.LocalDateTime;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;

@Slf4j
@Service
public class ApiTokenService {

    private final ApiTokenMapper apiTokenMapper;

    public ApiTokenService(ApiTokenMapper apiTokenMapper) {
        this.apiTokenMapper = apiTokenMapper;
    }

    public ApiTokenVO createToken(Long tenantId, Long userId, String tokenName, LocalDateTime expiresAt) {
        String rawToken = "forge_" + UUID.randomUUID().toString().replace("-", "");
        String tokenHash = sha256(rawToken);
        String tokenPrefix = rawToken.substring(0, 12);

        ApiTokenDO token = new ApiTokenDO();
        token.setTenantId(tenantId);
        token.setUserId(userId);
        token.setTokenName(tokenName);
        token.setTokenHash(tokenHash);
        token.setTokenPrefix(tokenPrefix);
        token.setExpiresAt(expiresAt);
        token.setStatus(1);
        apiTokenMapper.insert(token);

        log.info("创建 API Token: tenant={}, user={}, name={}", tenantId, userId, tokenName);
        return ApiTokenVO.builder()
                .id(token.getId())
                .tokenName(tokenName)
                .tokenPrefix(tokenPrefix)
                .expiresAt(expiresAt)
                .status(1)
                .rawToken(rawToken)
                .build();
    }

    public List<ApiTokenVO> listTokens(Long tenantId, Long userId) {
        List<ApiTokenDO> tokens = apiTokenMapper.selectList(new LambdaQueryWrapper<ApiTokenDO>()
                .eq(ApiTokenDO::getTenantId, tenantId)
                .eq(ApiTokenDO::getUserId, userId)
                .orderByDesc(ApiTokenDO::getGmtCreate));
        return tokens.stream().map(t -> ApiTokenVO.builder()
                .id(t.getId())
                .tokenName(t.getTokenName())
                .tokenPrefix(t.getTokenPrefix())
                .expiresAt(t.getExpiresAt())
                .status(t.getStatus())
                .build()).toList();
    }

    public void revokeToken(Long tokenId) {
        ApiTokenDO token = apiTokenMapper.selectById(tokenId);
        if (token != null) {
            token.setStatus(0);
            apiTokenMapper.updateById(token);
            log.info("吊销 API Token: id={}", tokenId);
        }
    }

    public ApiTokenDO validateApiToken(String rawToken) {
        String tokenHash = sha256(rawToken);
        ApiTokenDO token = apiTokenMapper.selectOne(new LambdaQueryWrapper<ApiTokenDO>()
                .eq(ApiTokenDO::getTokenHash, tokenHash)
                .eq(ApiTokenDO::getStatus, 1));
        if (token == null) return null;
        if (token.getExpiresAt() != null && token.getExpiresAt().isBefore(LocalDateTime.now())) {
            return null;
        }
        return token;
    }

    private String sha256(String input) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] hash = digest.digest(input.getBytes(StandardCharsets.UTF_8));
            return HexFormat.of().formatHex(hash);
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("SHA-256 不可用", e);
        }
    }
}
