package com.shulex.forge.pipeline.adapter.flow;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Data
@Component
@ConfigurationProperties(prefix = "forge.adapter.flow")
public class FlowConfig {
    private String baseUrl = "https://devops.aliyun.com";
    private String orgId;
    private String accessToken;
}
