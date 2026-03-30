package com.shulex.forge.engine.execution.ai;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.shulex.forge.engine.common.SysException;
import com.shulex.forge.engine.common.ErrorCode;
import lombok.extern.slf4j.Slf4j;
import okhttp3.*;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.util.concurrent.TimeUnit;

@Slf4j
@Component
public class ClaudeClient {

    private final ClaudeConfig config;
    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;

    public ClaudeClient(ClaudeConfig config) {
        this.config = config;
        this.httpClient = new OkHttpClient.Builder()
                .connectTimeout(30, TimeUnit.SECONDS)
                .readTimeout(120, TimeUnit.SECONDS)
                .writeTimeout(30, TimeUnit.SECONDS)
                .build();
        this.objectMapper = new ObjectMapper();
    }

    public AiResponse chat(String systemPrompt, String userMessage) {
        try {
            ObjectNode body = objectMapper.createObjectNode();
            body.put("model", config.getModel());
            body.put("max_tokens", config.getMaxTokens());
            body.put("system", systemPrompt);

            ArrayNode messages = body.putArray("messages");
            ObjectNode userMsg = messages.addObject();
            userMsg.put("role", "user");
            userMsg.put("content", userMessage);

            String url = config.getBaseUrl().endsWith("/")
                    ? config.getBaseUrl() + "v1/messages"
                    : config.getBaseUrl() + "/v1/messages";

            Request request = new Request.Builder()
                    .url(url)
                    .addHeader("x-api-key", config.getApiKey())
                    .addHeader("anthropic-version", "2023-06-01")
                    .addHeader("Content-Type", "application/json")
                    .post(RequestBody.create(objectMapper.writeValueAsString(body),
                            MediaType.parse("application/json")))
                    .build();

            long start = System.currentTimeMillis();
            try (Response response = httpClient.newCall(request).execute()) {
                long latency = System.currentTimeMillis() - start;
                if (!response.isSuccessful()) {
                    String errorBody = response.body() != null ? response.body().string() : "no body";
                    log.error("Claude API 失败: status={}, body={}", response.code(), errorBody);
                    throw new SysException(ErrorCode.AI_CALL_FAILED,
                            new RuntimeException("Claude API 返回 " + response.code()));
                }

                JsonNode root = objectMapper.readTree(response.body().string());
                String content = root.path("content").get(0).path("text").asText();
                long inputTokens = root.path("usage").path("input_tokens").asLong();
                long outputTokens = root.path("usage").path("output_tokens").asLong();
                String model = root.path("model").asText();
                String stopReason = root.path("stop_reason").asText();

                log.debug("Claude 调用完成: model={}, tokens={}+{}, latency={}ms",
                        model, inputTokens, outputTokens, latency);

                return new AiResponse(content, inputTokens, outputTokens, model, stopReason);
            }
        } catch (SysException e) {
            throw e;
        } catch (IOException e) {
            throw new SysException(ErrorCode.AI_CALL_FAILED, e);
        }
    }
}
