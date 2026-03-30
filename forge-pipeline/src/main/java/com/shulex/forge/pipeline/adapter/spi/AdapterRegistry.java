package com.shulex.forge.pipeline.adapter.spi;

import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

@Slf4j
@Component
public class AdapterRegistry {
    private final Map<String, CodeHostingAdapter> codeHostingAdapters = new ConcurrentHashMap<>();
    private final Map<String, ContainerOrchestrationAdapter> containerAdapters = new ConcurrentHashMap<>();
    private final Map<String, CiCdAdapter> ciCdAdapters = new ConcurrentHashMap<>();

    public AdapterRegistry(List<CodeHostingAdapter> codeHostingList,
                           List<ContainerOrchestrationAdapter> containerList,
                           List<CiCdAdapter> ciCdList) {
        codeHostingList.forEach(a -> {
            codeHostingAdapters.put(a.getType(), a);
            log.info("注册代码托管适配器: {}", a.getType());
        });
        containerList.forEach(a -> {
            containerAdapters.put(a.getType(), a);
            log.info("注册容器编排适配器: {}", a.getType());
        });
        ciCdList.forEach(a -> {
            ciCdAdapters.put(a.getType(), a);
            log.info("注册 CI/CD 适配器: {}", a.getType());
        });
    }

    public CodeHostingAdapter getCodeHostingAdapter(String type) {
        CodeHostingAdapter adapter = codeHostingAdapters.get(type);
        if (adapter == null) throw new IllegalArgumentException("未找到代码托管适配器: " + type);
        return adapter;
    }

    public ContainerOrchestrationAdapter getContainerAdapter(String type) {
        ContainerOrchestrationAdapter adapter = containerAdapters.get(type);
        if (adapter == null) throw new IllegalArgumentException("未找到容器编排适配器: " + type);
        return adapter;
    }

    public CiCdAdapter getCiCdAdapter(String type) {
        CiCdAdapter adapter = ciCdAdapters.get(type);
        if (adapter == null) throw new IllegalArgumentException("未找到 CI/CD 适配器: " + type);
        return adapter;
    }

    public Map<String, List<String>> getRegisteredAdapterTypes() {
        return Map.of(
                "codeHosting", List.copyOf(codeHostingAdapters.keySet()),
                "containerOrchestration", List.copyOf(containerAdapters.keySet()),
                "ciCd", List.copyOf(ciCdAdapters.keySet())
        );
    }
}
