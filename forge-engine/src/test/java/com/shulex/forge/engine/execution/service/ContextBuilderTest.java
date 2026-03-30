package com.shulex.forge.engine.execution.service;

import com.shulex.forge.engine.infrastructure.http.PipelineClient;
import com.shulex.forge.engine.infrastructure.http.SpecsClient;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.*;

class ContextBuilderTest {

    private ContextBuilder contextBuilder;
    private SpecsClient specsClient;
    private PipelineClient pipelineClient;

    @BeforeEach
    void setUp() {
        specsClient = mock(SpecsClient.class);
        pipelineClient = mock(PipelineClient.class);
        contextBuilder = new ContextBuilder(specsClient, pipelineClient);
    }

    @Test
    void buildContext_includesStandards() {
        when(specsClient.getStandards("java")).thenReturn("## Java 规范\n内容");
        when(specsClient.getReviewRules()).thenReturn("- 规则1");
        when(pipelineClient.listRepositoryTree(any(), any(), any(), any())).thenReturn(List.of());

        String context = contextBuilder.buildContext("codeup", "repo-123", "main", "创建用户服务");
        assertThat(context).contains("Java 规范");
    }

    @Test
    void buildContext_includesFileTree() {
        when(specsClient.getStandards("java")).thenReturn("");
        when(specsClient.getReviewRules()).thenReturn("");
        when(pipelineClient.listRepositoryTree("codeup", "repo-123", "/", "main"))
                .thenReturn(List.of("src/main/java/App.java", "pom.xml"));

        String context = contextBuilder.buildContext("codeup", "repo-123", "main", "test");
        assertThat(context).contains("src/main/java/App.java");
    }
}
