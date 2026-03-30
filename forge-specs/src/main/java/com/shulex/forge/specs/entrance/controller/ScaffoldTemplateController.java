package com.shulex.forge.specs.entrance.controller;

import com.shulex.forge.specs.common.Result;
import com.shulex.forge.specs.entrance.vo.ScaffoldTemplateVO;
import com.shulex.forge.specs.infrastructure.entity.ScaffoldTemplateDO;
import com.shulex.forge.specs.service.ScaffoldTemplateService;
import org.springframework.web.bind.annotation.*;
import java.util.List;

@RestController
@RequestMapping("/api/scaffolds")
public class ScaffoldTemplateController {

    private final ScaffoldTemplateService scaffoldTemplateService;

    public ScaffoldTemplateController(ScaffoldTemplateService scaffoldTemplateService) {
        this.scaffoldTemplateService = scaffoldTemplateService;
    }

    @GetMapping
    public Result<List<ScaffoldTemplateVO>> listActive() {
        List<ScaffoldTemplateDO> list = scaffoldTemplateService.listActive();
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    @GetMapping("/{name}")
    public Result<ScaffoldTemplateVO> getByName(@PathVariable("name") String name) {
        ScaffoldTemplateDO template = scaffoldTemplateService.getByName(name);
        return Result.ok(toVO(template));
    }

    private ScaffoldTemplateVO toVO(ScaffoldTemplateDO entity) {
        ScaffoldTemplateVO vo = new ScaffoldTemplateVO();
        vo.setId(entity.getId());
        vo.setName(entity.getName());
        vo.setDescription(entity.getDescription());
        vo.setTechStack(entity.getTechStack());
        vo.setTemplateContent(entity.getTemplateContent());
        return vo;
    }
}
