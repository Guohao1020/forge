package com.shulex.forge.specs.service;

import com.shulex.forge.specs.common.BizException;
import com.shulex.forge.specs.infrastructure.entity.PromptTemplateDO;
import com.shulex.forge.specs.infrastructure.mapper.PromptTemplateMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.ActiveProfiles;
import static org.assertj.core.api.Assertions.assertThat;
import org.springframework.transaction.annotation.Transactional;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

@SpringBootTest
@ActiveProfiles("test")
@Transactional
class PromptTemplateServiceTest {

    @Autowired
    private PromptTemplateService promptTemplateService;

    @Autowired
    private PromptTemplateMapper promptTemplateMapper;

    @Test
    void getActiveByKey_returnsActiveTemplate() {
        PromptTemplateDO t = new PromptTemplateDO();
        t.setTemplateKey("code-generation");
        t.setName("代码生成");
        t.setSystemPrompt("You are a code generator...");
        t.setStandardsInjection("Follow these standards: {{standards}}");
        t.setVersion(1);
        t.setIsActive(true);
        promptTemplateMapper.insert(t);

        PromptTemplateDO result = promptTemplateService.getActiveByKey("code-generation");
        assertThat(result.getName()).isEqualTo("代码生成");
        assertThat(result.getSystemPrompt()).contains("code generator");
    }

    @Test
    void getActiveByKey_throwsWhenNotFound() {
        assertThatThrownBy(() -> promptTemplateService.getActiveByKey("nonexistent"))
                .isInstanceOf(BizException.class);
    }

    @Test
    void listAll_returnsOnlyActiveTemplates() {
        PromptTemplateDO active = new PromptTemplateDO();
        active.setTemplateKey("test-generation");
        active.setName("测试生成");
        active.setSystemPrompt("...");
        active.setVersion(1);
        active.setIsActive(true);
        promptTemplateMapper.insert(active);

        PromptTemplateDO inactive = new PromptTemplateDO();
        inactive.setTemplateKey("test-generation");
        inactive.setName("测试生成 v0");
        inactive.setSystemPrompt("...");
        inactive.setVersion(0);
        inactive.setIsActive(false);
        promptTemplateMapper.insert(inactive);

        var result = promptTemplateService.listActive();
        assertThat(result.stream().noneMatch(t -> t.getVersion() == 0)).isTrue();
    }
}
