package com.shulex.forge.engine.infrastructure.http;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import okhttp3.*;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.IOException;

@Slf4j
@Component
public class SpecsClient {

    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;
    private final String baseUrl;

    public SpecsClient(@Value("${forge.specs.base-url}") String baseUrl) {
        this.baseUrl = baseUrl;
        this.httpClient = new OkHttpClient();
        this.objectMapper = new ObjectMapper();
    }

    public String getPromptTemplate(String templateKey) {
        try {
            Request request = new Request.Builder()
                    .url(baseUrl + "/api/prompts/" + templateKey)
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return null;
                JsonNode root = objectMapper.readTree(response.body().string());
                return root.path("data").path("systemPrompt").asText(null);
            }
        } catch (IOException e) {
            log.warn("获取 Prompt 模板失败: key={}", templateKey, e);
            return null;
        }
    }

    public String getStandards(String category) {
        try {
            Request request = new Request.Builder()
                    .url(baseUrl + "/api/standards?category=" + category)
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return "";
                JsonNode root = objectMapper.readTree(response.body().string());
                StringBuilder sb = new StringBuilder();
                for (JsonNode item : root.path("data")) {
                    sb.append("## ").append(item.path("title").asText()).append("\n");
                    sb.append(item.path("content").asText()).append("\n\n");
                }
                return sb.toString();
            }
        } catch (IOException e) {
            log.warn("获取编码规范失败: category={}", category, e);
            return "";
        }
    }

    public String getReviewRules() {
        try {
            Request request = new Request.Builder()
                    .url(baseUrl + "/api/review-rules")
                    .build();
            try (Response response = httpClient.newCall(request).execute()) {
                if (!response.isSuccessful()) return "";
                JsonNode root = objectMapper.readTree(response.body().string());
                StringBuilder sb = new StringBuilder();
                for (JsonNode item : root.path("data")) {
                    sb.append("- [").append(item.path("severity").asText()).append("] ");
                    sb.append(item.path("name").asText()).append(": ");
                    sb.append(item.path("description").asText()).append("\n");
                }
                return sb.toString();
            }
        } catch (IOException e) {
            log.warn("获取 Review 规则失败", e);
            return "";
        }
    }
}
