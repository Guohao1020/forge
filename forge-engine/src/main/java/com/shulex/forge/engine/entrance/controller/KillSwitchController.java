package com.shulex.forge.engine.entrance.controller;

import com.shulex.forge.engine.common.Result;
import com.shulex.forge.engine.orchestration.model.KillSwitchLevel;
import com.shulex.forge.engine.orchestration.service.KillSwitchService;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

@RestController
@RequestMapping("/api/killswitch")
public class KillSwitchController {

    private final KillSwitchService killSwitchService;

    public KillSwitchController(KillSwitchService killSwitchService) {
        this.killSwitchService = killSwitchService;
    }

    @GetMapping
    public Result<Map<String, String>> getStatus() {
        return Result.ok(Map.of("level", killSwitchService.getLevel().name()));
    }

    @PostMapping("/activate")
    public Result<Void> activate(@RequestParam("level") String level) {
        killSwitchService.activate(KillSwitchLevel.valueOf(level));
        return Result.ok(null);
    }

    @PostMapping("/deactivate")
    public Result<Void> deactivate() {
        killSwitchService.deactivate();
        return Result.ok(null);
    }
}
