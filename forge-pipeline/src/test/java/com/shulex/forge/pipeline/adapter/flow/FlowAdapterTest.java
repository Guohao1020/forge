package com.shulex.forge.pipeline.adapter.flow;

import com.shulex.forge.pipeline.adapter.model.PipelineInfo;
import com.shulex.forge.pipeline.adapter.model.PipelineRunInfo;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import java.io.IOException;
import java.util.List;
import static org.assertj.core.api.Assertions.assertThat;

class FlowAdapterTest {
    private MockWebServer mockServer;
    private FlowAdapter adapter;

    @BeforeEach
    void setUp() throws IOException {
        mockServer = new MockWebServer();
        mockServer.start();
        FlowConfig config = new FlowConfig();
        config.setBaseUrl(mockServer.url("/").toString());
        config.setOrgId("test-org");
        config.setAccessToken("test-token");
        adapter = new FlowAdapter(new FlowClient(config));
    }

    @AfterEach
    void tearDown() throws IOException { mockServer.shutdown(); }

    @Test
    void getType_returnsFlow() {
        assertThat(adapter.getType()).isEqualTo("flow");
    }

    @Test
    void listPipelines_convertToModel() {
        mockServer.enqueue(new MockResponse()
                .setBody("{\"result\":[{\"id\":\"p-1\",\"name\":\"build-pipeline\",\"status\":\"active\"}]}")
                .setResponseCode(200));
        List<PipelineInfo> list = adapter.listPipelines("test-org");
        assertThat(list).hasSize(1);
        assertThat(list.get(0).getName()).isEqualTo("build-pipeline");
    }

    @Test
    void triggerPipeline_returnsRunInfo() {
        mockServer.enqueue(new MockResponse()
                .setBody("{\"result\":{\"pipelineRunId\":\"r-1\",\"pipelineId\":\"p-1\",\"status\":\"RUNNING\"}}")
                .setResponseCode(200));
        PipelineRunInfo run = adapter.triggerPipeline("test-org", "p-1", "main");
        assertThat(run.getRunId()).isEqualTo("r-1");
        assertThat(run.getStatus()).isEqualTo("running");
    }

    @Test
    void getPipelineRun_returnsStatus() {
        mockServer.enqueue(new MockResponse()
                .setBody("{\"result\":{\"pipelineRunId\":\"r-1\",\"pipelineId\":\"p-1\",\"status\":\"SUCCESS\"}}")
                .setResponseCode(200));
        PipelineRunInfo run = adapter.getPipelineRun("test-org", "p-1", "r-1");
        assertThat(run.getStatus()).isEqualTo("success");
    }

    @Test
    void getPipelineRunLogs_returnsLogText() {
        mockServer.enqueue(new MockResponse()
                .setBody("Build started...\nCompiling...\nBuild success.")
                .setResponseCode(200));
        String logs = adapter.getPipelineRunLogs("test-org", "p-1", "r-1");
        assertThat(logs).contains("Build success");
    }
}
