package com.shulex.forge.engine.execution.listener;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.shulex.forge.engine.execution.model.StepRequest;
import com.shulex.forge.engine.execution.model.StepResult;
import com.shulex.forge.engine.execution.service.StepExecutor;
import com.shulex.forge.engine.infrastructure.config.KafkaConfig;
import lombok.extern.slf4j.Slf4j;
import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.kafka.core.KafkaTemplate;
import org.springframework.stereotype.Component;

@Slf4j
@Component
public class StepRequestListener {

    private final StepExecutor stepExecutor;
    private final KafkaTemplate<String, String> kafkaTemplate;
    private final ObjectMapper objectMapper = new ObjectMapper();

    public StepRequestListener(StepExecutor stepExecutor, KafkaTemplate<String, String> kafkaTemplate) {
        this.stepExecutor = stepExecutor;
        this.kafkaTemplate = kafkaTemplate;
    }

    @KafkaListener(topics = KafkaConfig.TOPIC_STEP_REQUEST, groupId = "forge-engine-executor")
    public void onStepRequest(String message) {
        try {
            StepRequest request = objectMapper.readValue(message, StepRequest.class);
            log.info("收到步骤请求: task={}, step={}, type={}",
                    request.getTaskId(), request.getStepId(), request.getStepType());

            StepResult result = stepExecutor.execute(request);

            String resultJson = objectMapper.writeValueAsString(result);
            kafkaTemplate.send(KafkaConfig.TOPIC_STEP_RESULT, String.valueOf(request.getTaskId()), resultJson);
        } catch (Exception e) {
            log.error("处理步骤请求失败", e);
        }
    }
}
