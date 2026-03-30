package com.shulex.forge.specs.entrance.controller;

import com.shulex.forge.specs.common.Result;
import com.shulex.forge.specs.entrance.vo.PromptTemplateVO;
import com.shulex.forge.specs.infrastructure.entity.PromptTemplateDO;
import com.shulex.forge.specs.service.PromptTemplateService;
import org.springframework.web.bind.annotation.*;
import java.util.List;

@RestController
@RequestMapping("/api/prompts")
public class PromptTemplateController {

    private final PromptTemplateService promptTemplateService;

    public PromptTemplateController(PromptTemplateService promptTemplateService) {
        this.promptTemplateService = promptTemplateService;
    }

    @GetMapping
    public Result<List<PromptTemplateVO>> listActive() {
        List<PromptTemplateDO> list = promptTemplateService.listActive();
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    @GetMapping("/{templateKey}")
    public Result<PromptTemplateVO> getByKey(@PathVariable("templateKey") String templateKey) {
        PromptTemplateDO template = promptTemplateService.getActiveByKey(templateKey);
        return Result.ok(toVO(template));
    }

    private PromptTemplateVO toVO(PromptTemplateDO entity) {
        PromptTemplateVO vo = new PromptTemplateVO();
        vo.setId(entity.getId());
        vo.setTemplateKey(entity.getTemplateKey());
        vo.setName(entity.getName());
        vo.setDescription(entity.getDescription());
        vo.setSystemPrompt(entity.getSystemPrompt());
        vo.setStandardsInjection(entity.getStandardsInjection());
        vo.setVersion(entity.getVersion());
        return vo;
    }
}
