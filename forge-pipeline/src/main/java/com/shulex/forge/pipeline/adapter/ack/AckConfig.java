package com.shulex.forge.pipeline.adapter.ack;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

@Data
@Component
@ConfigurationProperties(prefix = "forge.adapter.ack")
public class AckConfig {
    private String kubeConfigPath;
}
