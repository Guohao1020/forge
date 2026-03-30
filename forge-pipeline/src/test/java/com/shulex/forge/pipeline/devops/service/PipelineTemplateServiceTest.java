package com.shulex.forge.pipeline.devops.service;

import com.shulex.forge.pipeline.devops.model.ProjectType;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class PipelineTemplateServiceTest {

    private final PipelineTemplateService service = new PipelineTemplateService();

    @Test
    void generateTemplate_javaService_containsCompileAndTest() {
        String yaml = service.generateTemplate(ProjectType.JAVA_SERVICE, "forge-engine", "main");
        assertThat(yaml).contains("mvn clean compile");
        assertThat(yaml).contains("mvn test");
        assertThat(yaml).contains("docker build");
    }

    @Test
    void generateTemplate_javaService_containsImagePush() {
        String yaml = service.generateTemplate(ProjectType.JAVA_SERVICE, "forge-engine", "main");
        assertThat(yaml).contains("docker push");
    }

    @Test
    void generateTemplate_javaService_containsProjectName() {
        String yaml = service.generateTemplate(ProjectType.JAVA_SERVICE, "my-service", "develop");
        assertThat(yaml).contains("my-service");
    }
}
