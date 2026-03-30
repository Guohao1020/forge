package com.shulex.forge.engine.infrastructure.config;

import org.apache.kafka.clients.admin.NewTopic;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class KafkaConfig {

    public static final String TOPIC_STEP_REQUEST = "forge-engine-step-request";
    public static final String TOPIC_STEP_RESULT = "forge-engine-step-result";

    @Bean
    public NewTopic stepRequestTopic() {
        return new NewTopic(TOPIC_STEP_REQUEST, 3, (short) 1);
    }

    @Bean
    public NewTopic stepResultTopic() {
        return new NewTopic(TOPIC_STEP_RESULT, 3, (short) 1);
    }
}
