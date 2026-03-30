package com.shulex.forge.pipeline.entrance.controller;

import com.shulex.forge.pipeline.adapter.spi.AdapterRegistry;
import com.shulex.forge.pipeline.common.Result;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.Map;

@RestController
@RequestMapping("/api/adapters")
public class AdapterHealthController {

    private final AdapterRegistry adapterRegistry;

    public AdapterHealthController(AdapterRegistry adapterRegistry) {
        this.adapterRegistry = adapterRegistry;
    }

    @GetMapping("/health")
    public Result<Map<String, Object>> health() {
        return Result.ok(Map.of(
                "status", "UP",
                "registeredAdapters", adapterRegistry.getRegisteredAdapterTypes()
        ));
    }
}
