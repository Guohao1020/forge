package com.shulex.forge.pipeline.adapter.spi;

import com.shulex.forge.pipeline.adapter.model.PipelineCreateRequest;
import com.shulex.forge.pipeline.adapter.model.PipelineInfo;
import com.shulex.forge.pipeline.adapter.model.PipelineRunInfo;
import java.util.List;

public interface CiCdAdapter {
    String getType();
    PipelineInfo createPipeline(String orgId, PipelineCreateRequest request);
    List<PipelineInfo> listPipelines(String orgId);
    PipelineInfo getPipeline(String orgId, String pipelineId);
    PipelineRunInfo triggerPipeline(String orgId, String pipelineId, String branch);
    PipelineRunInfo getPipelineRun(String orgId, String pipelineId, String runId);
    List<PipelineRunInfo> listPipelineRuns(String orgId, String pipelineId, int limit);
    String getPipelineRunLogs(String orgId, String pipelineId, String runId);
}
