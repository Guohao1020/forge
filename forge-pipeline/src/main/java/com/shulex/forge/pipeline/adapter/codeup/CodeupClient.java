package com.shulex.forge.pipeline.adapter.codeup;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.pipeline.infrastructure.http.RetryableHttpClient;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import java.util.Map;

@Slf4j
@Component
public class CodeupClient {
    private final CodeupConfig config;
    private final RetryableHttpClient httpClient;
    private final ObjectMapper objectMapper;

    public CodeupClient(CodeupConfig config) {
        this.config = config;
        this.httpClient = new RetryableHttpClient(3, 500);
        this.objectMapper = new ObjectMapper();
    }

    private Map<String, String> headers() {
        return Map.of("Private-Token", config.getAccessToken());
    }

    private String baseUrl() {
        String url = config.getBaseUrl();
        return url.endsWith("/") ? url.substring(0, url.length() - 1) : url;
    }

    public JsonNode listRepositoryTree(String repoId, String path, String ref) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/tree?path=%s&ref=%s",
                baseUrl(), config.getOrgId(), repoId, path, ref);
        return parseResult(httpClient.get(url, headers()));
    }

    public JsonNode getFileBlobs(String repoId, String filePath, String ref) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/blobs?filePath=%s&ref=%s",
                baseUrl(), config.getOrgId(), repoId, filePath, ref);
        return parseResult(httpClient.get(url, headers()));
    }

    public JsonNode createCommit(String repoId, String branch, String message, String actionsJson) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/commits",
                baseUrl(), config.getOrgId(), repoId);
        try {
            var payload = objectMapper.createObjectNode()
                    .put("branch", branch).put("commitMessage", message)
                    .set("actions", objectMapper.readTree(actionsJson));
            return parseResult(httpClient.post(url, objectMapper.writeValueAsString(payload), headers()));
        } catch (Exception e) { throw new RuntimeException("构建提交请求失败", e); }
    }

    public JsonNode createBranch(String repoId, String branchName, String ref) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/branches",
                baseUrl(), config.getOrgId(), repoId);
        try {
            var payload = objectMapper.createObjectNode().put("branchName", branchName).put("ref", ref);
            return parseResult(httpClient.post(url, objectMapper.writeValueAsString(payload), headers()));
        } catch (Exception e) { throw new RuntimeException("构建分支请求失败", e); }
    }

    public void deleteBranch(String repoId, String branchName) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/branches?branchName=%s",
                baseUrl(), config.getOrgId(), repoId, branchName);
        httpClient.delete(url, headers());
    }

    public JsonNode getBranch(String repoId, String branchName) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/branches/%s",
                baseUrl(), config.getOrgId(), repoId, branchName);
        return parseResult(httpClient.get(url, headers()));
    }

    public JsonNode listBranches(String repoId) {
        String url = String.format("%s/api/v4/projects/%s/%s/repository/branches",
                baseUrl(), config.getOrgId(), repoId);
        return parseResult(httpClient.get(url, headers()));
    }

    public JsonNode createMergeRequest(String repoId, String title, String description,
                                        String sourceBranch, String targetBranch) {
        String url = String.format("%s/api/v4/projects/%s/%s/merge_requests",
                baseUrl(), config.getOrgId(), repoId);
        try {
            var payload = objectMapper.createObjectNode()
                    .put("title", title).put("description", description)
                    .put("sourceBranch", sourceBranch).put("targetBranch", targetBranch);
            return parseResult(httpClient.post(url, objectMapper.writeValueAsString(payload), headers()));
        } catch (Exception e) { throw new RuntimeException("构建 MR 请求失败", e); }
    }

    public JsonNode getMergeRequest(String repoId, Long mrId) {
        String url = String.format("%s/api/v4/projects/%s/%s/merge_requests/%d",
                baseUrl(), config.getOrgId(), repoId, mrId);
        return parseResult(httpClient.get(url, headers()));
    }

    public void mergeMergeRequest(String repoId, Long mrId) {
        String url = String.format("%s/api/v4/projects/%s/%s/merge_requests/%d/merge",
                baseUrl(), config.getOrgId(), repoId, mrId);
        httpClient.put(url, "{}", headers());
    }

    public void closeMergeRequest(String repoId, Long mrId) {
        String url = String.format("%s/api/v4/projects/%s/%s/merge_requests/%d",
                baseUrl(), config.getOrgId(), repoId, mrId);
        try {
            var payload = objectMapper.createObjectNode().put("state", "closed");
            httpClient.put(url, objectMapper.writeValueAsString(payload), headers());
        } catch (Exception e) { throw new RuntimeException("关闭 MR 失败", e); }
    }

    public void addComment(String repoId, Long mrId, String comment) {
        String url = String.format("%s/api/v4/projects/%s/%s/merge_requests/%d/comments",
                baseUrl(), config.getOrgId(), repoId, mrId);
        try {
            var payload = objectMapper.createObjectNode().put("content", comment);
            httpClient.post(url, objectMapper.writeValueAsString(payload), headers());
        } catch (Exception e) { throw new RuntimeException("添加 MR 评论失败", e); }
    }

    public JsonNode createWebhook(String repoId, String url, String secretToken, String events) {
        String reqUrl = String.format("%s/api/v4/projects/%s/%s/webhooks",
                baseUrl(), config.getOrgId(), repoId);
        try {
            var payload = objectMapper.createObjectNode()
                    .put("url", url).put("secretToken", secretToken).put("events", events);
            return parseResult(httpClient.post(reqUrl, objectMapper.writeValueAsString(payload), headers()));
        } catch (Exception e) { throw new RuntimeException("创建 Webhook 失败", e); }
    }

    public JsonNode listWebhooks(String repoId) {
        String url = String.format("%s/api/v4/projects/%s/%s/webhooks",
                baseUrl(), config.getOrgId(), repoId);
        return parseResult(httpClient.get(url, headers()));
    }

    public void deleteWebhook(String repoId, Long webhookId) {
        String url = String.format("%s/api/v4/projects/%s/%s/webhooks/%d",
                baseUrl(), config.getOrgId(), repoId, webhookId);
        httpClient.delete(url, headers());
    }

    private JsonNode parseResult(String body) {
        try {
            JsonNode root = objectMapper.readTree(body);
            return root.has("result") ? root.get("result") : root;
        } catch (Exception e) { throw new RuntimeException("JSON 解析失败: " + body, e); }
    }
}
