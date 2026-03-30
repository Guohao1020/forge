package com.shulex.forge.engine.infrastructure.http;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.shulex.forge.engine.common.ErrorCode;
import com.shulex.forge.engine.common.SysException;
import com.shulex.forge.engine.execution.model.GeneratedCode;
import lombok.extern.slf4j.Slf4j;
import okhttp3.*;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

@Slf4j
@Component
public class PipelineClient {

    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;
    private final String baseUrl;

    public PipelineClient(@Value("${forge.pipeline.base-url}") String baseUrl) {
        this.baseUrl = baseUrl;
        this.httpClient = new OkHttpClient();
        this.objectMapper = new ObjectMapper();
    }

    public String getFileContent(String adapterType, String repoId, String filePath, String ref) {
        try {
            String url = baseUrl + "/api/adapters/" + adapterType + "/repos/" + repoId
                    + "/files?path=" + filePath + "&ref=" + ref;
            Request request = new Request.Builder().url(url).build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return null;
                JsonNode root = objectMapper.readTree(response.body().string());
                return root.path("data").path("content").asText(null);
            }
        } catch (IOException e) {
            log.warn("获取文件内容失败: repo={}, path={}", repoId, filePath, e);
            return null;
        }
    }

    public List<String> listRepositoryTree(String adapterType, String repoId, String path, String ref) {
        try {
            String url = baseUrl + "/api/adapters/" + adapterType + "/repos/" + repoId
                    + "/tree?path=" + path + "&ref=" + ref;
            Request request = new Request.Builder().url(url).build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return List.of();
                JsonNode root = objectMapper.readTree(response.body().string());
                List<String> paths = new ArrayList<>();
                for (JsonNode item : root.path("data")) {
                    paths.add(item.path("path").asText());
                }
                return paths;
            }
        } catch (IOException e) {
            log.warn("获取文件树失败: repo={}", repoId, e);
            return List.of();
        }
    }

    public String createBranch(String adapterType, String repoId, String branchName, String ref) {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("branchName", branchName);
            body.put("ref", ref);
            String url = baseUrl + "/api/adapters/" + adapterType + "/repos/" + repoId + "/branches";
            Request request = new Request.Builder()
                    .url(url)
                    .post(RequestBody.create(objectMapper.writeValueAsString(body),
                            MediaType.parse("application/json")))
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return null;
                JsonNode root = objectMapper.readTree(response.body().string());
                return root.path("data").path("name").asText(null);
            }
        } catch (IOException e) {
            log.warn("创建分支失败: repo={}, branch={}", repoId, branchName, e);
            return null;
        }
    }

    public String commitFiles(String adapterType, String repoId, String branch,
                              String commitMessage, List<GeneratedCode> files) {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("branch", branch);
            body.put("commitMessage", commitMessage);
            ArrayNode filesArray = body.putArray("files");
            for (GeneratedCode file : files) {
                ObjectNode f = filesArray.addObject();
                f.put("filePath", file.getFilePath());
                f.put("content", file.getContent());
                f.put("action", file.getAction());
            }
            String url = baseUrl + "/api/adapters/" + adapterType + "/repos/" + repoId + "/commits";
            Request request = new Request.Builder()
                    .url(url)
                    .post(RequestBody.create(objectMapper.writeValueAsString(body),
                            MediaType.parse("application/json")))
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) {
                    throw new SysException(ErrorCode.CODE_COMMIT_FAILED,
                            new RuntimeException("提交失败: " + response.code()));
                }
                JsonNode root = objectMapper.readTree(response.body().string());
                return root.path("data").asText(null);
            }
        } catch (SysException e) {
            throw e;
        } catch (IOException e) {
            throw new SysException(ErrorCode.CODE_COMMIT_FAILED, e);
        }
    }

    public Long createMergeRequest(String adapterType, String repoId,
                                    String sourceBranch, String targetBranch,
                                    String title, String description) {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("sourceBranch", sourceBranch);
            body.put("targetBranch", targetBranch);
            body.put("title", title);
            body.put("description", description);
            String url = baseUrl + "/api/adapters/" + adapterType + "/repos/" + repoId + "/merge-requests";
            Request request = new Request.Builder()
                    .url(url)
                    .post(RequestBody.create(objectMapper.writeValueAsString(body),
                            MediaType.parse("application/json")))
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return null;
                JsonNode root = objectMapper.readTree(response.body().string());
                return root.path("data").path("id").asLong();
            }
        } catch (IOException e) {
            log.warn("创建 MR 失败", e);
            return null;
        }
    }
}
