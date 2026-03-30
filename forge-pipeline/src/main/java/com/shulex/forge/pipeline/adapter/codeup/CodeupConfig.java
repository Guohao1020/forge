package com.shulex.forge.pipeline.adapter.codeup;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Data
@Component
@ConfigurationProperties(prefix = "forge.adapter.codeup")
public class CodeupConfig {
    private String baseUrl = "https://codeup.aliyun.com";
    private String orgId;
    private String accessToken;
}
