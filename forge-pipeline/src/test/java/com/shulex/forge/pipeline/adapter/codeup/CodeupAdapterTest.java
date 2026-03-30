package com.shulex.forge.pipeline.adapter.codeup;

import com.shulex.forge.pipeline.adapter.model.*;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.core.io.ClassPathResource;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.List;
import static org.assertj.core.api.Assertions.assertThat;

class CodeupAdapterTest {
    private MockWebServer mockServer;
    private CodeupAdapter adapter;

    @BeforeEach
    void setUp() throws IOException {
        mockServer = new MockWebServer();
        mockServer.start();
        CodeupConfig config = new CodeupConfig();
        config.setBaseUrl(mockServer.url("/").toString());
        config.setOrgId("test-org");
        config.setAccessToken("test-token");
        adapter = new CodeupAdapter(new CodeupClient(config));
    }

    @AfterEach
    void tearDown() throws IOException { mockServer.shutdown(); }

    @Test
    void getType_returnsCcodeup() {
        assertThat(adapter.getType()).isEqualTo("codeup");
    }

    @Test
    void listRepositoryTree_convertsToModel() throws Exception {
        String body = new ClassPathResource("mock/codeup-tree-response.json").getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));
        List<FileTreeNode> nodes = adapter.listRepositoryTree("repo-1", "/", "main");
        assertThat(nodes).hasSize(2);
        assertThat(nodes.get(0).getType()).isEqualTo("tree");
        assertThat(nodes.get(1).getName()).isEqualTo("pom.xml");
    }

    @Test
    void getFileContent_convertsToModel() throws Exception {
        String body = new ClassPathResource("mock/codeup-file-response.json").getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));
        FileContent content = adapter.getFileContent("repo-1", "pom.xml", "main");
        assertThat(content.getPath()).isEqualTo("pom.xml");
    }

    @Test
    void createCommitWithMultipleFiles_returnsCommitId() throws Exception {
        String body = new ClassPathResource("mock/codeup-commit-response.json").getContentAsString(StandardCharsets.UTF_8);
        mockServer.enqueue(new MockResponse().setBody(body).setResponseCode(200));
        List<CommitFile> files = List.of(
                CommitFile.builder().path("README.md").content("# Hello").action("create").build()
        );
        String commitId = adapter.createCommitWithMultipleFiles("repo-1", "main", "test", files);
        assertThat(commitId).isEqualTo("abc123def456");
    }
}
