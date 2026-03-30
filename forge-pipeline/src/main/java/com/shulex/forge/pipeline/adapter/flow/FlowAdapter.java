package com.shulex.forge.pipeline.adapter.flow;

import com.fasterxml.jackson.databind.JsonNode;
import com.shulex.forge.pipeline.adapter.model.PipelineCreateRequest;
import com.shulex.forge.pipeline.adapter.model.PipelineInfo;
import com.shulex.forge.pipeline.adapter.model.PipelineRunInfo;
import com.shulex.forge.pipeline.adapter.spi.CiCdAdapter;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import java.util.ArrayList;
import java.util.List;

@Slf4j
@Component
public class FlowAdapter implements CiCdAdapter {
    private final FlowClient client;

    public FlowAdapter(FlowClient client) { this.client = client; }

    @Override public String getType() { return "flow"; }

    @Override
    public PipelineInfo createPipeline(String orgId, PipelineCreateRequest request) {
        JsonNode node = client.createPipeline(orgId, request.getName(),
                request.getRepoUrl(), request.getBranch(), request.getYamlContent());
        return PipelineInfo.builder().id(node.path("id").asText())
                .name(node.path("name").asText()).status("active").build();
    }

    @Override
    public List<PipelineInfo> listPipelines(String orgId) {
        JsonNode result = client.listPipelines(orgId);
        List<PipelineInfo> list = new ArrayList<>();
        for (JsonNode node : result) {
            list.add(PipelineInfo.builder().id(node.path("id").asText())
                    .name(node.path("name").asText()).status(node.path("status").asText()).build());
        }
        return list;
    }

    @Override
    public PipelineInfo getPipeline(String orgId, String pipelineId) {
        JsonNode node = client.getPipeline(orgId, pipelineId);
        return PipelineInfo.builder().id(node.path("id").asText())
                .name(node.path("name").asText()).status(node.path("status").asText()).build();
    }

    @Override
    public PipelineRunInfo triggerPipeline(String orgId, String pipelineId, String branch) {
        return toRunInfo(client.triggerPipeline(orgId, pipelineId, branch));
    }

    @Override
    public PipelineRunInfo getPipelineRun(String orgId, String pipelineId, String runId) {
        return toRunInfo(client.getPipelineRun(orgId, pipelineId, runId));
    }

    @Override
    public List<PipelineRunInfo> listPipelineRuns(String orgId, String pipelineId, int limit) {
        JsonNode result = client.listPipelineRuns(orgId, pipelineId, limit);
        List<PipelineRunInfo> list = new ArrayList<>();
        for (JsonNode node : result) list.add(toRunInfo(node));
        return list;
    }

    @Override
    public String getPipelineRunLogs(String orgId, String pipelineId, String runId) {
        return client.getPipelineRunLogs(orgId, pipelineId, runId);
    }

    private PipelineRunInfo toRunInfo(JsonNode node) {
        return PipelineRunInfo.builder().runId(node.path("pipelineRunId").asText())
                .pipelineId(node.path("pipelineId").asText())
                .status(node.path("status").asText().toLowerCase()).build();
    }
}
