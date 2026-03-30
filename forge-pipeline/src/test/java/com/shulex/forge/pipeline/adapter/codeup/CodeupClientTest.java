package com.shulex.forge.pipeline.adapter.codeup;

import com.fasterxml.jackson.databind.JsonNode;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.core.io.ClassPathResource;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import static org.assertj.core.api.Assertions.assertThat;

class CodeupClientTest {
    private MockWebServer mockServer;
    private CodeupClient client;

    @BeforeEach
    void setUp() throws IOException {
        mockServer = new MockWebServer();
        mockServer.start();
        CodeupConfig config = new CodeupConfig();
        config.setBaseUrl(mockServer.url("/").toString());
        config.setOrgId("test-org");
        config.setAccessToken("test-token");
        client = new CodeupClient(config);
    }

    @AfterEach
    void tearDown() throws IOException { mockServer.shutdown(); }

    @Test
    void listRepositoryTree_parsesResponse() throws Exception {
        String body = new ClassPathResource("mock/codeup-tree-response.json").getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));
        JsonNode result = client.listRepositoryTree("repo-1", "/", "main");
        assertThat(result.isArray()).isTrue();
        assertThat(result.size()).isEqualTo(2);
    }

    @Test
    void getFileBlobs_parsesResponse() throws Exception {
        String body = new ClassPathResource("mock/codeup-file-response.json").getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));
        JsonNode result = client.getFileBlobs("repo-1", "pom.xml", "main");
        assertThat(result.get("filePath").asText()).isEqualTo("pom.xml");
    }

    @Test
    void createCommitWithMultipleFiles_returnsCommitId() throws Exception {
        String body = new ClassPathResource("mock/codeup-commit-response.json").getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));
        JsonNode result = client.createCommit("repo-1", "main", "test commit", "[]");
        assertThat(result.get("commitId").asText()).isEqualTo("abc123def456");
    }

    @Test
    void request_sendsAccessTokenHeader() throws Exception {
        mockServer.enqueue(new MockResponse().setBody("{\"result\":[]}").setResponseCode(200));
        client.listRepositoryTree("repo-1", "/", "main");
        var request = mockServer.takeRequest();
        assertThat(request.getHeader("Private-Token")).isEqualTo("test-token");
    }
}
