package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.infrastructure.entity.ModelCallLogDO;
import com.shulex.forge.engine.infrastructure.mapper.ModelCallLogMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

@Slf4j
@Service
public class TokenUsageService {

    private final ModelCallLogMapper modelCallLogMapper;

    public TokenUsageService(ModelCallLogMapper modelCallLogMapper) {
        this.modelCallLogMapper = modelCallLogMapper;
    }

    public void recordCall(Long taskId, Long stepId, String modelId, String purpose,
                           long inputTokens, long outputTokens, long latencyMs) {
        ModelCallLogDO logDO = new ModelCallLogDO();
        logDO.setTaskId(taskId);
        logDO.setStepId(stepId);
        logDO.setModelId(modelId);
        logDO.setPurpose(purpose);
        logDO.setInputTokens(inputTokens);
        logDO.setOutputTokens(outputTokens);
        logDO.setLatencyMs(latencyMs);
        logDO.setIsFallback(0);
        modelCallLogMapper.insert(logDO);
        log.debug("记录模型调用: task={}, model={}, tokens={}+{}", taskId, modelId, inputTokens, outputTokens);
    }
}
