"""Tests for profile scan file selection and dimension matching."""

import pytest
from src.activities.profile import (
    _matches_dimension,
    _select_files_for_dimension,
    ALL_DIMENSIONS,
    DIMENSION_FILE_PATTERNS,
)


class TestDimensionPatterns:
    def test_all_dimensions_have_patterns(self):
        for dim in ALL_DIMENSIONS:
            assert dim in DIMENSION_FILE_PATTERNS, f"Missing patterns for {dim}"
            assert len(DIMENSION_FILE_PATTERNS[dim]) > 0, f"Empty patterns for {dim}"

    def test_matches_dimension_api_catalog(self):
        # "controller" is substring of "controllers/auth.controller.ts"
        assert _matches_dimension("src/controllers/auth.controller.ts", ["controller"])
        # "routes" is substring of "routes/api.go"
        assert _matches_dimension("routes/api.go", ["routes"])
        # "router.go" matches "router.go" exactly
        assert _matches_dimension("internal/router.go", ["router.go"])
        # "handler.go" matches "handler.go" file
        assert _matches_dimension("api/handler.go", ["handler.go"])
        assert not _matches_dimension("README.md", ["handler.go", "controller"])

    def test_matches_dimension_db_schema(self):
        # "migration" substring in "migrations/001_init.sql"
        assert _matches_dimension("migrations/001_init.sql", ["migration"])
        # "model.go" matches path containing "model.go"
        assert _matches_dimension("internal/user/model.go", ["model.go"])
        # ".sql" matches any SQL file
        assert _matches_dimension("schema/create_tables.sql", [".sql"])
        assert not _matches_dimension("config.yaml", ["migration", "schema", "model.go"])

    def test_matches_dimension_case_insensitive(self):
        assert _matches_dimension("src/Router.go", ["router.go"])
        assert _matches_dimension("Controller.ts", ["controller"])


class TestFileSelection:
    def test_select_files_string_entries(self):
        """File tree entries can be plain strings (not dicts)."""
        file_tree = [
            "internal/router.go",       # matches "router.go"
            "api/handler.go",           # matches "handler.go"
            "README.md",
            "go.mod",
        ]
        result = _select_files_for_dimension(file_tree, "api_catalog")
        assert "internal/router.go" in result
        assert "api/handler.go" in result
        assert "README.md" not in result

    def test_select_files_dict_entries(self):
        """File tree entries can be dicts with path field."""
        file_tree = [
            {"path": "internal/router.go", "type": "file"},
            {"path": "README.md", "type": "file"},
        ]
        result = _select_files_for_dimension(file_tree, "api_catalog")
        assert "internal/router.go" in result

    def test_select_files_skips_binary(self):
        file_tree = [
            "assets/logo.png",
            "fonts/inter.woff2",
            "handler.go",
        ]
        result = _select_files_for_dimension(file_tree, "api_catalog")
        assert "assets/logo.png" not in result
        assert "fonts/inter.woff2" not in result

    def test_select_files_max_limit(self):
        """Should return at most MAX_FILES_PER_DIMENSION files."""
        file_tree = [f"handler_{i}.go" for i in range(50)]
        result = _select_files_for_dimension(file_tree, "api_catalog")
        assert len(result) <= 15  # MAX_FILES_PER_DIMENSION

    def test_select_files_empty_tree(self):
        result = _select_files_for_dimension([], "api_catalog")
        assert result == []

    def test_select_files_no_matches(self):
        file_tree = ["README.md", "LICENSE", ".gitignore"]
        result = _select_files_for_dimension(file_tree, "api_catalog")
        assert result == []
