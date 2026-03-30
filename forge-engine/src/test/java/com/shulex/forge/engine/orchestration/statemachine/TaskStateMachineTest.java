package com.shulex.forge.engine.orchestration.statemachine;

import com.shulex.forge.engine.orchestration.model.TaskStatus;
import org.junit.jupiter.api.Test;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class TaskStateMachineTest {

    @Test
    void transition_submittedToAnalyzing() {
        assertThat(TaskStateMachine.transition(TaskStatus.SUBMITTED, TaskStatus.ANALYZING)).isTrue();
    }

    @Test
    void transition_analyzingToPlanning() {
        assertThat(TaskStateMachine.transition(TaskStatus.ANALYZING, TaskStatus.PLANNING)).isTrue();
    }

    @Test
    void transition_fullHappyPath() {
        TaskStatus[] path = {
            TaskStatus.SUBMITTED, TaskStatus.ANALYZING, TaskStatus.PLANNING,
            TaskStatus.GENERATING, TaskStatus.REVIEWING, TaskStatus.DEPLOYING, TaskStatus.DONE
        };
        for (int i = 0; i < path.length - 1; i++) {
            assertThat(TaskStateMachine.transition(path[i], path[i + 1])).isTrue();
        }
    }

    @Test
    void transition_reviewingToHumanReview() {
        assertThat(TaskStateMachine.transition(TaskStatus.REVIEWING, TaskStatus.HUMAN_REVIEW)).isTrue();
    }

    @Test
    void transition_invalidTransitionReturnsFalse() {
        assertThat(TaskStateMachine.transition(TaskStatus.DONE, TaskStatus.SUBMITTED)).isFalse();
    }

    @Test
    void transition_anyToFailed() {
        assertThat(TaskStateMachine.transition(TaskStatus.GENERATING, TaskStatus.FAILED)).isTrue();
        assertThat(TaskStateMachine.transition(TaskStatus.REVIEWING, TaskStatus.FAILED)).isTrue();
    }

    @Test
    void transition_anyToCancelled() {
        assertThat(TaskStateMachine.transition(TaskStatus.PLANNING, TaskStatus.CANCELLED)).isTrue();
    }
}
