package com.shulex.forge.identity.service;

import com.shulex.forge.identity.common.BizException;
import com.shulex.forge.identity.common.ErrorCode;
import io.jsonwebtoken.*;
import io.jsonwebtoken.security.Keys;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Service;

import javax.crypto.SecretKey;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.Date;
import java.util.List;
import java.util.UUID;

@Slf4j
@Service
public class TokenService {

    private final SecretKey secretKey;
    private final long accessTokenExpireMinutes;
    private final long refreshTokenExpireMinutes;
    private final StringRedisTemplate redisTemplate;
    private static final String BLACKLIST_PREFIX = "forge:token:blacklist:";

    public TokenService(
            @Value("${forge.jwt.secret}") String secret,
            @Value("${forge.jwt.access-token-expire-minutes}") long accessTokenExpireMinutes,
            @Value("${forge.jwt.refresh-token-expire-minutes}") long refreshTokenExpireMinutes,
            StringRedisTemplate redisTemplate) {
        this.secretKey = Keys.hmacShaKeyFor(secret.getBytes(StandardCharsets.UTF_8));
        this.accessTokenExpireMinutes = accessTokenExpireMinutes;
        this.refreshTokenExpireMinutes = refreshTokenExpireMinutes;
        this.redisTemplate = redisTemplate;
    }

    public String generateAccessToken(Long userId, Long tenantId, String username, List<String> roles) {
        Date now = new Date();
        Date expiry = new Date(now.getTime() + accessTokenExpireMinutes * 60 * 1000);
        return Jwts.builder()
                .id(UUID.randomUUID().toString())
                .subject(username)
                .claim("userId", userId)
                .claim("tenantId", tenantId)
                .claim("roles", roles)
                .claim("type", "access")
                .issuedAt(now)
                .expiration(expiry)
                .signWith(secretKey)
                .compact();
    }

    public String generateRefreshToken(Long userId, Long tenantId, String username) {
        Date now = new Date();
        Date expiry = new Date(now.getTime() + refreshTokenExpireMinutes * 60 * 1000);
        return Jwts.builder()
                .id(UUID.randomUUID().toString())
                .subject(username)
                .claim("userId", userId)
                .claim("tenantId", tenantId)
                .claim("type", "refresh")
                .issuedAt(now)
                .expiration(expiry)
                .signWith(secretKey)
                .compact();
    }

    public Claims parseToken(String token) {
        try {
            return Jwts.parser()
                    .verifyWith(secretKey)
                    .build()
                    .parseSignedClaims(token)
                    .getPayload();
        } catch (ExpiredJwtException e) {
            throw new BizException(ErrorCode.TOKEN_EXPIRED);
        } catch (JwtException e) {
            throw new BizException(ErrorCode.TOKEN_INVALID);
        }
    }

    public Claims validateToken(String token) {
        Claims claims = parseToken(token);
        String jti = claims.getId();
        String blacklistKey = BLACKLIST_PREFIX + (jti != null ? jti : token.hashCode());
        if (Boolean.TRUE.equals(redisTemplate.hasKey(blacklistKey))) {
            throw new BizException(ErrorCode.TOKEN_REVOKED);
        }
        return claims;
    }

    public void revokeToken(String token) {
        try {
            Claims claims = parseToken(token);
            String jti = claims.getId();
            String blacklistKey = BLACKLIST_PREFIX + (jti != null ? jti : token.hashCode());
            long ttlMs = claims.getExpiration().getTime() - System.currentTimeMillis();
            if (ttlMs > 0) {
                redisTemplate.opsForValue().set(blacklistKey, "revoked", Duration.ofMillis(ttlMs));
                log.info("Token 已吊销: user={}", claims.getSubject());
            }
        } catch (BizException e) {
            log.debug("吊销已过期 Token，忽略: {}", e.getMessage());
        }
    }
}
