package com.shulex.forge.engine.orchestration.model;

public enum TaskStatus {
    SUBMITTED, ANALYZING, PLANNING, GENERATING, REVIEWING,
    HUMAN_REVIEW, APPROVED, REJECTED, DEPLOYING, DONE,
    FAILED, CANCELLED
}
