package com.shulex.forge.engine.orchestration.service;

import com.shulex.forge.engine.orchestration.model.RiskLevel;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class RiskAssessorTest {

    private final RiskAssessor riskAssessor = new RiskAssessor();

    @Test
    void initialAssess_lowRiskForSimpleGeneration() {
        assertThat(riskAssessor.initialAssess("创建一个简单的 CRUD 接口", "GENERATE"))
                .isEqualTo(RiskLevel.LOW);
    }

    @Test
    void initialAssess_highRiskForSecurityRelated() {
        assertThat(riskAssessor.initialAssess("修改支付模块的加密逻辑", "ITERATE"))
                .isEqualTo(RiskLevel.HIGH);
    }

    @Test
    void finalAssess_upgradesWhenReviewScoreLow() {
        assertThat(riskAssessor.finalAssess(RiskLevel.LOW, 85, 12))
                .isEqualTo(RiskLevel.HIGH);
    }

    @Test
    void finalAssess_keepsLowWhenAllGood() {
        assertThat(riskAssessor.finalAssess(RiskLevel.LOW, 95, 3))
                .isEqualTo(RiskLevel.LOW);
    }
}
