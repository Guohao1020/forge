"""Tests for the reviewer-side prompt + parser machinery.

Spec: §2.9.3.c-d.
"""

from __future__ import annotations

import pytest

from src.openharness.engine.prompts import (
    REVIEWER_DIFF_MAX_BYTES,
    REVIEWER_SYSTEM_PROMPT,
    ReviewerParseError,
    build_reviewer_prompt,
    parse_verdict,
)


# ---------------------------------------------------------------------------
# build_reviewer_prompt — substring invariants
# ---------------------------------------------------------------------------


def test_build_reviewer_prompt_substring_invariants():
    """All three input strings appear in the rendered prompt under
    their labeled sections."""
    rendered = build_reviewer_prompt(
        summary="Added a /api/health endpoint that returns 200 OK.",
        current_diff="diff --git a/api.go b/api.go\n+func health() {}",
        original_request="please add a health endpoint",
    )

    # Section headers
    assert "User's original request" in rendered
    assert "Agent's summary of work" in rendered
    assert "Git diff" in rendered

    # Inputs themselves
    assert "please add a health endpoint" in rendered
    assert "Added a /api/health endpoint that returns 200 OK." in rendered
    assert "diff --git a/api.go b/api.go" in rendered

    # The closing instruction
    assert "APPROVE" in rendered
    assert "REVISE" in rendered
    assert "REJECT" in rendered


def test_build_reviewer_prompt_deterministic():
    """Same inputs -> same output. No timestamps, no random IDs."""
    args = dict(
        summary="s",
        current_diff="d",
        original_request="r",
    )
    a = build_reviewer_prompt(**args)
    b = build_reviewer_prompt(**args)
    assert a == b


def test_reviewer_system_prompt_constant_present():
    """The pinned system prompt constant exists and contains the
    verdict vocabulary."""
    assert "senior engineer" in REVIEWER_SYSTEM_PROMPT.lower() or "reviewer" in REVIEWER_SYSTEM_PROMPT.lower()
    assert "APPROVE" in REVIEWER_SYSTEM_PROMPT
    assert "REVISE" in REVIEWER_SYSTEM_PROMPT
    assert "REJECT" in REVIEWER_SYSTEM_PROMPT


def test_reviewer_diff_max_bytes_constant():
    """The cap is 32 KiB per spec §2.9.3.e."""
    assert REVIEWER_DIFF_MAX_BYTES == 32_768


# ---------------------------------------------------------------------------
# parse_verdict — happy paths
# ---------------------------------------------------------------------------


def test_parse_verdict_approve():
    verdict, details = parse_verdict("APPROVE")
    assert verdict == "APPROVE"
    assert details == ""


def test_parse_verdict_revise_with_details():
    verdict, details = parse_verdict("REVISE add null check on line 42")
    assert verdict == "REVISE"
    assert details == "add null check on line 42"


def test_parse_verdict_reject_with_reason():
    verdict, details = parse_verdict(
        "REJECT the diff implements a different feature than asked"
    )
    assert verdict == "REJECT"
    assert "different feature than asked" in details


def test_parse_verdict_finds_verdict_in_middle_of_response():
    """The regex is multiline; preamble or chatter before the
    verdict line is allowed."""
    verdict, details = parse_verdict(
        "Let me think...\nLooking at the diff, I see...\n"
        "APPROVE\n"
        "Some trailing text that should be ignored."
    )
    assert verdict == "APPROVE"


def test_parse_verdict_first_line_is_verdict():
    verdict, details = parse_verdict("REVISE rename the function\nmore notes here")
    assert verdict == "REVISE"
    assert details == "rename the function"


# ---------------------------------------------------------------------------
# parse_verdict — error paths
# ---------------------------------------------------------------------------


def test_parse_verdict_invalid_raises_error():
    """Response with no recognized verdict line raises
    ReviewerParseError. The exception message includes a snippet of
    the response so the caller can log it."""
    with pytest.raises(ReviewerParseError) as exc_info:
        parse_verdict("This response has no verdict at all.")
    assert "This response has no verdict" in str(exc_info.value)


def test_parse_verdict_empty_input_raises():
    with pytest.raises(ReviewerParseError):
        parse_verdict("")


def test_parse_verdict_lowercase_verdict_does_not_match():
    """Verdicts are case-sensitive — the spec uses uppercase as the
    parsing contract. A lowercase 'approve' is not parsed."""
    with pytest.raises(ReviewerParseError):
        parse_verdict("approve")
