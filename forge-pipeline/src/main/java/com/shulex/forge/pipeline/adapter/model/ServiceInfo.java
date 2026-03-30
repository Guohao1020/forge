package com.shulex.forge.pipeline.adapter.model;
import lombok.Builder;
import lombok.Data;
import java.util.Map;
@Data
@Builder
public class ServiceInfo {
    private String namespace;
    private String name;
    private String type;
    private Map<String, String> selector;
    private Integer port;
    private Integer targetPort;
    private String clusterIp;
}
