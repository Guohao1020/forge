package com.shulex.forge.pipeline.infrastructure.http;

import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class RetryableHttpClientTest {

    private MockWebServer mockServer;
    private RetryableHttpClient client;

    @BeforeEach
    void setUp() throws IOException {
        mockServer = new MockWebServer();
        mockServer.start();
        client = new RetryableHttpClient(3, 100); // maxAttempts=3, retryDelayMs=100
    }

    @AfterEach
    void tearDown() throws IOException {
        mockServer.shutdown();
    }

    @Test
    void get_returnsBody() throws Exception {
        mockServer.enqueue(new MockResponse().setBody("{\"result\":\"ok\"}").setResponseCode(200));
        String result = client.get(mockServer.url("/test").toString(), null);
        assertThat(result).contains("ok");
    }

    @Test
    void get_retriesOnServerError() throws Exception {
        mockServer.enqueue(new MockResponse().setResponseCode(503));
        mockServer.enqueue(new MockResponse().setResponseCode(503));
        mockServer.enqueue(new MockResponse().setBody("{\"ok\":true}").setResponseCode(200));
        String result = client.get(mockServer.url("/retry").toString(), null);
        assertThat(result).contains("ok");
        assertThat(mockServer.getRequestCount()).isEqualTo(3);
    }

    @Test
    void get_throwsAfterMaxRetries() {
        mockServer.enqueue(new MockResponse().setResponseCode(503));
        mockServer.enqueue(new MockResponse().setResponseCode(503));
        mockServer.enqueue(new MockResponse().setResponseCode(503));
        assertThatThrownBy(() -> client.get(mockServer.url("/fail").toString(), null))
                .isInstanceOf(RuntimeException.class);
    }

    @Test
    void post_sendsBodyAndReturnsResult() throws Exception {
        mockServer.enqueue(new MockResponse().setBody("{\"id\":1}").setResponseCode(200));
        String result = client.post(mockServer.url("/create").toString(), "{\"name\":\"test\"}", null);
        assertThat(result).contains("id");
    }
}
