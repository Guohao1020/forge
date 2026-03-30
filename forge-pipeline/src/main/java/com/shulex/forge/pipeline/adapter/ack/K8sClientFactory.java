package com.shulex.forge.pipeline.adapter.ack;

import io.kubernetes.client.openapi.ApiClient;
import io.kubernetes.client.openapi.apis.AppsV1Api;
import io.kubernetes.client.openapi.apis.CoreV1Api;
import io.kubernetes.client.util.Config;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.io.FileReader;
import java.io.IOException;

@Slf4j
@Component
public class K8sClientFactory {
    private final AckConfig ackConfig;

    public K8sClientFactory(AckConfig ackConfig) {
        this.ackConfig = ackConfig;
    }

    public ApiClient createClient() {
        try {
            String path = ackConfig.getKubeConfigPath();
            if (path != null && !path.isBlank()) {
                return Config.fromConfig(new FileReader(path));
            }
            return Config.defaultClient();
        } catch (IOException e) {
            throw new RuntimeException("无法创建 K8s 客户端", e);
        }
    }

    public CoreV1Api coreV1Api() { return new CoreV1Api(createClient()); }
    public AppsV1Api appsV1Api() { return new AppsV1Api(createClient()); }
}
