package com.shulex.forge.specs.entrance.controller;

import com.shulex.forge.specs.common.Result;
import com.shulex.forge.specs.entrance.vo.StandardVO;
import com.shulex.forge.specs.infrastructure.entity.StandardDO;
import com.shulex.forge.specs.service.StandardService;
import org.springframework.web.bind.annotation.*;
import java.util.List;

@RestController
@RequestMapping("/api/standards")
public class StandardController {

    private final StandardService standardService;

    public StandardController(StandardService standardService) {
        this.standardService = standardService;
    }

    @GetMapping
    public Result<List<StandardVO>> listByCategory(@RequestParam("category") String category) {
        List<StandardDO> list = standardService.listByCategory(category);
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    @GetMapping("/effective")
    public Result<List<StandardVO>> listEffective(
            @RequestParam("category") String category,
            @RequestParam(value = "scopeLevel", defaultValue = "company") String scopeLevel,
            @RequestParam(value = "scopeId", required = false) String scopeId) {
        List<StandardDO> list = standardService.listEffective(category, scopeLevel, scopeId);
        return Result.ok(list.stream().map(this::toVO).toList());
    }

    private StandardVO toVO(StandardDO entity) {
        StandardVO vo = new StandardVO();
        vo.setId(entity.getId());
        vo.setCategory(entity.getCategory());
        vo.setTitle(entity.getTitle());
        vo.setContent(entity.getContent());
        vo.setScopeLevel(entity.getScopeLevel());
        return vo;
    }
}
