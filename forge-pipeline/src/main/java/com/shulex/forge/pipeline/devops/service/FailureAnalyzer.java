package com.shulex.forge.pipeline.devops.service;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import okhttp3.*;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.io.IOException;
import java.util.Map;

@Slf4j
@Service
public class FailureAnalyzer {

    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;
    private final String engineBaseUrl;

    public FailureAnalyzer(@Value("${forge.engine.base-url}") String engineBaseUrl) {
        this.httpClient = new OkHttpClient();
        this.objectMapper = new ObjectMapper();
        this.engineBaseUrl = engineBaseUrl;
    }

    public String analyzeLogs(String buildLogs) {
        try {
            String body = objectMapper.writeValueAsString(Map.of(
                    "tenantId", 1,
                    "userId", 1,
                    "requirement", "分析以下构建失败日志并给出修复建议：\n\n" + buildLogs,
                    "taskType", "ITERATE",
                    "repoId", "analysis-only"
            ));

            Request request = new Request.Builder()
                    .url(engineBaseUrl + "/api/tasks")
                    .post(RequestBody.create(body, MediaType.parse("application/json")))
                    .build();

            try (Response response = httpClient.newCall(request).execute()) {
                if (response.isSuccessful() && response.body() != null) {
                    JsonNode root = objectMapper.readTree(response.body().string());
                    Long taskId = root.path("data").path("id").asLong();
                    log.info("失败分析任务已创建: taskId={}", taskId);
                    return "Analysis task created: " + taskId;
                }
            }
        } catch (IOException e) {
            log.error("失败分析请求失败", e);
        }
        return "Failed to create analysis task";
    }
}
