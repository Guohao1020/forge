package com.shulex.forge.pipeline.infrastructure.credential;

import lombok.extern.slf4j.Slf4j;
import org.springframework.core.env.Environment;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class CredentialService {

    private final Environment environment;

    public CredentialService(Environment environment) {
        this.environment = environment;
    }

    public String getCredential(String key) {
        String value = environment.getProperty(key);
        if (value == null || value.isBlank()) {
            log.error("凭证未配置: {}", key);
            throw new IllegalStateException("凭证未配置: " + key);
        }
        return value;
    }

    public String getCredential(String key, String defaultValue) {
        return environment.getProperty(key, defaultValue);
    }
}
