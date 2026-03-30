package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.orchestration.model.RiskLevel;
import org.springframework.stereotype.Service;

import java.util.Set;

@Service
public class RiskAssessor {

    private static final Set<String> HIGH_RISK_KEYWORDS = Set.of(
            "支付", "加密", "权限", "安全", "密码", "token", "secret",
            "payment", "security", "auth", "credential", "DROP", "DELETE"
    );

    public RiskLevel initialAssess(String requirement, String taskType) {
        String lower = requirement.toLowerCase();
        for (String keyword : HIGH_RISK_KEYWORDS) {
            if (lower.contains(keyword.toLowerCase())) {
                return RiskLevel.HIGH;
            }
        }
        return "ITERATE".equals(taskType) ? RiskLevel.MEDIUM : RiskLevel.LOW;
    }

    public RiskLevel finalAssess(RiskLevel initialRisk, int reviewScore, int fileCount) {
        if (reviewScore < 90 || fileCount > 10) {
            return RiskLevel.HIGH;
        }
        if (initialRisk == RiskLevel.HIGH) {
            return RiskLevel.HIGH;
        }
        if (fileCount > 5 || reviewScore < 95) {
            return RiskLevel.MEDIUM;
        }
        return initialRisk;
    }
}
