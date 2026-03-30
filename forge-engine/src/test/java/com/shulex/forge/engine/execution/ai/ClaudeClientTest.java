package com.shulex.forge.engine.execution.ai;

import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import okhttp3.mockwebserver.RecordedRequest;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class ClaudeClientTest {

    private MockWebServer server;
    private ClaudeClient claudeClient;

    @BeforeEach
    void setUp() throws Exception {
        server = new MockWebServer();
        server.start();
        ClaudeConfig config = new ClaudeConfig();
        config.setApiKey("test-key");
        config.setModel("claude-sonnet-4-20250514");
        config.setBaseUrl(server.url("/").toString());
        config.setMaxTokens(4096);
        claudeClient = new ClaudeClient(config);
    }

    @AfterEach
    void tearDown() throws Exception {
        server.shutdown();
    }

    @Test
    void chat_sendsCorrectRequest() throws Exception {
        server.enqueue(new MockResponse()
                .setBody("{\"content\":[{\"type\":\"text\",\"text\":\"Hello\"}],\"model\":\"claude-sonnet-4-20250514\",\"stop_reason\":\"end_turn\",\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}")
                .addHeader("Content-Type", "application/json"));

        AiResponse response = claudeClient.chat("system prompt", "user message");
        assertThat(response.getContent()).isEqualTo("Hello");
        assertThat(response.getInputTokens()).isEqualTo(10);
        assertThat(response.getOutputTokens()).isEqualTo(5);

        RecordedRequest request = server.takeRequest();
        assertThat(request.getHeader("x-api-key")).isEqualTo("test-key");
        assertThat(request.getHeader("anthropic-version")).isEqualTo("2023-06-01");
    }

    @Test
    void chat_handlesLargeResponse() throws Exception {
        String longText = "x".repeat(1000);
        server.enqueue(new MockResponse()
                .setBody("{\"content\":[{\"type\":\"text\",\"text\":\"" + longText + "\"}],\"model\":\"claude-sonnet-4-20250514\",\"stop_reason\":\"end_turn\",\"usage\":{\"input_tokens\":100,\"output_tokens\":200}}")
                .addHeader("Content-Type", "application/json"));

        AiResponse response = claudeClient.chat("sys", "msg");
        assertThat(response.getContent()).hasSize(1000);
    }
}
