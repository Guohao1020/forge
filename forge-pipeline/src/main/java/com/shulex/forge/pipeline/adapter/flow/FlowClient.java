package com.shulex.forge.pipeline.adapter.flow;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.pipeline.infrastructure.http.RetryableHttpClient;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.util.Map;

@Slf4j
@Component
public class FlowClient {
    private final FlowConfig config;
    private final RetryableHttpClient httpClient;
    private final ObjectMapper objectMapper;

    public FlowClient(FlowConfig config) {
        this.config = config;
        this.httpClient = new RetryableHttpClient(3, 500);
        this.objectMapper = new ObjectMapper();
    }

    private Map<String, String> headers() {
        return Map.of("x-devops-token", config.getAccessToken());
    }

    private String baseUrl() {
        String url = config.getBaseUrl();
        return url.endsWith("/") ? url.substring(0, url.length() - 1) : url;
    }

    public JsonNode createPipeline(String orgId, String name, String repoUrl, String branch, String yamlContent) {
        String url = String.format("%s/oapi/v1/flow/pipelines?orgId=%s", baseUrl(), orgId);
        try {
            var payload = objectMapper.createObjectNode()
                    .put("name", name).put("repoUrl", repoUrl)
                    .put("branch", branch).put("yamlContent", yamlContent);
            return parseResult(httpClient.post(url, objectMapper.writeValueAsString(payload), headers()));
        } catch (Exception e) { throw new RuntimeException("创建流水线失败", e); }
    }

    public JsonNode listPipelines(String orgId) {
        String url = String.format("%s/oapi/v1/flow/pipelines?orgId=%s", baseUrl(), orgId);
        return parseResult(httpClient.get(url, headers()));
    }

    public JsonNode getPipeline(String orgId, String pipelineId) {
        String url = String.format("%s/oapi/v1/flow/pipelines/%s?orgId=%s", baseUrl(), pipelineId, orgId);
        return parseResult(httpClient.get(url, headers()));
    }

    public JsonNode triggerPipeline(String orgId, String pipelineId, String branch) {
        String url = String.format("%s/oapi/v1/flow/pipelines/%s/runs?orgId=%s", baseUrl(), pipelineId, orgId);
        try {
            var payload = objectMapper.createObjectNode();
            payload.putArray("branchModeBranchs").add(branch);
            return parseResult(httpClient.post(url, objectMapper.writeValueAsString(payload), headers()));
        } catch (Exception e) { throw new RuntimeException("触发流水线失败", e); }
    }

    public JsonNode getPipelineRun(String orgId, String pipelineId, String runId) {
        String url = String.format("%s/oapi/v1/flow/pipelines/%s/runs/%s?orgId=%s",
                baseUrl(), pipelineId, runId, orgId);
        return parseResult(httpClient.get(url, headers()));
    }

    public JsonNode listPipelineRuns(String orgId, String pipelineId, int limit) {
        String url = String.format("%s/oapi/v1/flow/pipelines/%s/runs?orgId=%s&maxResults=%d",
                baseUrl(), pipelineId, orgId, limit);
        return parseResult(httpClient.get(url, headers()));
    }

    public String getPipelineRunLogs(String orgId, String pipelineId, String runId) {
        String url = String.format("%s/oapi/v1/flow/pipelines/%s/runs/%s/logs?orgId=%s",
                baseUrl(), pipelineId, runId, orgId);
        return httpClient.get(url, headers());
    }

    private JsonNode parseResult(String body) {
        try {
            JsonNode root = objectMapper.readTree(body);
            return root.has("result") ? root.get("result") : root;
        } catch (Exception e) { throw new RuntimeException("JSON 解析失败", e); }
    }
}
