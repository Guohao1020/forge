from ..api.usage import UsageSnapshot


class CostTracker:
    """Accumulates token usage across multiple API calls."""

    def __init__(self) -> None:
        self._input_tokens: int = 0
        self._output_tokens: int = 0

    def add(self, usage: UsageSnapshot) -> None:
        self._input_tokens += usage.input_tokens
        self._output_tokens += usage.output_tokens

    @property
    def total(self) -> UsageSnapshot:
        return UsageSnapshot(
            input_tokens=self._input_tokens,
            output_tokens=self._output_tokens,
        )

    def reset(self) -> None:
        self._input_tokens = 0
        self._output_tokens = 0
