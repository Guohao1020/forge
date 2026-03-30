package com.shulex.forge.specs.eval;

import com.shulex.forge.specs.service.PromptTemplateService;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.core.io.ClassPathResource;
import org.springframework.test.context.ActiveProfiles;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

import static org.assertj.core.api.Assertions.assertThat;

@SpringBootTest
@ActiveProfiles("test")
class PromptEvalTest {

    @Autowired
    private PromptTemplateService promptTemplateService;

    @ParameterizedTest
    @ValueSource(strings = {
            "requirement-analysis", "code-generation", "code-review",
            "test-generation", "fix-generation", "doc-generation"
    })
    void eachTemplate_hasGoodSample(String templateKey) throws IOException {
        String ext = "requirement-analysis".equals(templateKey) ? ".txt" : ".java";
        String path = "eval/" + templateKey + "/good-sample-01" + ext;
        ClassPathResource resource = new ClassPathResource(path);
        assertThat(resource.exists())
                .as("Good sample should exist for template: " + templateKey)
                .isTrue();
        String content = resource.getContentAsString(StandardCharsets.UTF_8);
        assertThat(content).isNotBlank();
    }

    @ParameterizedTest
    @ValueSource(strings = {
            "requirement-analysis", "code-generation", "code-review",
            "test-generation", "fix-generation", "doc-generation"
    })
    void eachTemplate_hasBadSample(String templateKey) throws IOException {
        String ext = "requirement-analysis".equals(templateKey) ? ".txt" : ".java";
        String path = "eval/" + templateKey + "/bad-sample-01" + ext;
        ClassPathResource resource = new ClassPathResource(path);
        assertThat(resource.exists())
                .as("Bad sample should exist for template: " + templateKey)
                .isTrue();
        String content = resource.getContentAsString(StandardCharsets.UTF_8);
        assertThat(content).isNotBlank();
    }

    @Test
    void goodSample_followsNamingConventions() throws IOException {
        String good = new ClassPathResource("eval/code-generation/good-sample-01.java")
                .getContentAsString(StandardCharsets.UTF_8);
        assertThat(good).doesNotContain("@Autowired");
        assertThat(good).contains("Result<");
    }

    @Test
    void badSample_violatesConventions() throws IOException {
        String bad = new ClassPathResource("eval/code-generation/bad-sample-01.java")
                .getContentAsString(StandardCharsets.UTF_8);
        boolean hasViolation = bad.contains("@Autowired")
                || bad.contains("System.out")
                || !bad.contains("Result<");
        assertThat(hasViolation).isTrue();
    }
}
