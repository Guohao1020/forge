package com.shulex.forge.engine.orchestration.statemachine;

import com.shulex.forge.engine.orchestration.model.TaskStatus;
import java.util.Map;
import java.util.Set;

public class TaskStateMachine {

    private static final Map<TaskStatus, Set<TaskStatus>> TRANSITIONS = Map.ofEntries(
        Map.entry(TaskStatus.SUBMITTED, Set.of(TaskStatus.ANALYZING, TaskStatus.FAILED, TaskStatus.CANCELLED)),
        Map.entry(TaskStatus.ANALYZING, Set.of(TaskStatus.PLANNING, TaskStatus.FAILED, TaskStatus.CANCELLED)),
        Map.entry(TaskStatus.PLANNING, Set.of(TaskStatus.GENERATING, TaskStatus.FAILED, TaskStatus.CANCELLED)),
        Map.entry(TaskStatus.GENERATING, Set.of(TaskStatus.REVIEWING, TaskStatus.FAILED, TaskStatus.CANCELLED)),
        Map.entry(TaskStatus.REVIEWING, Set.of(TaskStatus.HUMAN_REVIEW, TaskStatus.DEPLOYING, TaskStatus.FAILED, TaskStatus.CANCELLED)),
        Map.entry(TaskStatus.HUMAN_REVIEW, Set.of(TaskStatus.APPROVED, TaskStatus.REJECTED)),
        Map.entry(TaskStatus.APPROVED, Set.of(TaskStatus.DEPLOYING, TaskStatus.FAILED)),
        Map.entry(TaskStatus.REJECTED, Set.of()),
        Map.entry(TaskStatus.DEPLOYING, Set.of(TaskStatus.DONE, TaskStatus.FAILED)),
        Map.entry(TaskStatus.DONE, Set.of()),
        Map.entry(TaskStatus.FAILED, Set.of()),
        Map.entry(TaskStatus.CANCELLED, Set.of())
    );

    public static boolean transition(TaskStatus from, TaskStatus to) {
        Set<TaskStatus> allowed = TRANSITIONS.get(from);
        return allowed != null && allowed.contains(to);
    }

    private TaskStateMachine() {}
}
