package com.shulex.forge.engine.execution.ai;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Data
@Component
@ConfigurationProperties(prefix = "forge.claude")
public class ClaudeConfig {
    private String apiKey;
    private String model = "claude-sonnet-4-20250514";
    private String baseUrl = "https://api.anthropic.com";
    private int maxTokens = 4096;
}
