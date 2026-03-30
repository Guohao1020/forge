package com.shulex.forge.pipeline.infrastructure.http;

import io.github.resilience4j.ratelimiter.RateLimiter;
import io.github.resilience4j.ratelimiter.RateLimiterConfig;
import lombok.extern.slf4j.Slf4j;
import okhttp3.*;

import java.io.IOException;
import java.time.Duration;
import java.util.Map;
import java.util.concurrent.TimeUnit;

@Slf4j
public class RetryableHttpClient {

    private final OkHttpClient httpClient;
    private final int maxAttempts;
    private final long retryDelayMs;
    private final RateLimiter rateLimiter;

    public RetryableHttpClient(int maxAttempts, long retryDelayMs) {
        this.maxAttempts = maxAttempts;
        this.retryDelayMs = retryDelayMs;
        this.httpClient = new OkHttpClient.Builder()
                .connectTimeout(10, TimeUnit.SECONDS)
                .readTimeout(30, TimeUnit.SECONDS)
                .writeTimeout(30, TimeUnit.SECONDS)
                .build();
        this.rateLimiter = RateLimiter.of("http-client",
                RateLimiterConfig.custom()
                        .limitForPeriod(50)
                        .limitRefreshPeriod(Duration.ofSeconds(1))
                        .timeoutDuration(Duration.ofSeconds(5))
                        .build());
    }

    public String get(String url, Map<String, String> headers) {
        Request.Builder builder = new Request.Builder().url(url);
        if (headers != null) headers.forEach(builder::addHeader);
        return executeWithRetry(builder.build());
    }

    public String post(String url, String jsonBody, Map<String, String> headers) {
        RequestBody body = RequestBody.create(jsonBody, MediaType.parse("application/json; charset=utf-8"));
        Request.Builder builder = new Request.Builder().url(url).post(body);
        if (headers != null) headers.forEach(builder::addHeader);
        return executeWithRetry(builder.build());
    }

    public String put(String url, String jsonBody, Map<String, String> headers) {
        RequestBody body = RequestBody.create(jsonBody, MediaType.parse("application/json; charset=utf-8"));
        Request.Builder builder = new Request.Builder().url(url).put(body);
        if (headers != null) headers.forEach(builder::addHeader);
        return executeWithRetry(builder.build());
    }

    public void delete(String url, Map<String, String> headers) {
        Request.Builder builder = new Request.Builder().url(url).delete();
        if (headers != null) headers.forEach(builder::addHeader);
        executeWithRetry(builder.build());
    }

    private String executeWithRetry(Request request) {
        int attempt = 0;
        while (true) {
            attempt++;
            rateLimiter.acquirePermission();
            try (Response response = httpClient.newCall(request).execute()) {
                if (response.isSuccessful()) {
                    ResponseBody responseBody = response.body();
                    return responseBody != null ? responseBody.string() : "";
                }
                if (response.code() >= 500 && attempt < maxAttempts) {
                    log.warn("请求失败 ({}), 第 {}/{} 次尝试: {} {}",
                            response.code(), attempt, maxAttempts, request.method(), request.url());
                    sleep(retryDelayMs * attempt);
                    continue;
                }
                throw new RuntimeException("HTTP 请求失败: " + response.code()
                        + " " + request.method() + " " + request.url());
            } catch (IOException e) {
                if (attempt >= maxAttempts) {
                    throw new RuntimeException("HTTP 请求异常: " + request.url(), e);
                }
                log.warn("请求 IO 异常, 第 {}/{} 次尝试: {}", attempt, maxAttempts, e.getMessage());
                sleep(retryDelayMs * attempt);
            }
        }
    }

    private void sleep(long ms) {
        try { Thread.sleep(ms); } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
    }
}
