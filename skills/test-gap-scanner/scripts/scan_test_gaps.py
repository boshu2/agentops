#!/usr/bin/env python3
"""Scan Python codebases to identify modules without corresponding test files.

Enforces Law 7 (TDD) by identifying testing gaps before they become technical debt.

Usage:
    python scan_test_gaps.py .                          # Scan current directory
    python scan_test_gaps.py ./src --test-dir ./tests   # Custom test directory
    python scan_test_gaps.py . --format json            # JSON output for CI
    python scan_test_gaps.py . --min-coverage 80        # Fail if below threshold
    python scan_test_gaps.py ~/workspaces --all-repos   # Scan all git repos
"""
from __future__ import annotations

import argparse
import json
import sys
from dataclasses import dataclass, field
from pathlib import Path


# Default patterns to exclude from scanning
DEFAULT_EXCLUDES = [
    "**/tests/**",
    "**/test/**",
    "**/__pycache__/**",
    "**/.venv/**",
    "**/venv/**",
    "**/node_modules/**",
    "**/migrations/**",
    "**/site-packages/**",
    "**/.git/**",
    "**/build/**",
    "**/dist/**",
    "**/*.egg-info/**",
]

# Repos to exclude from --all-repos scanning
REPO_EXCLUDES = [
    "kubic-cm",  # Read-only upstream
    "dify",  # External fork
    ".git",
    "node_modules",
    "__pycache__",
    ".venv",
    "venv",
    # Backup and reference directories
    ".gitops-backup-20251120",
    "_work-refs",
    "_personal-refs",
]


def discover_git_repos(root_dir: Path, max_depth: int = 3) -> list[Path]:
    """Discover all git repositories under a root directory.

    Args:
        root_dir: Root directory to search
        max_depth: Maximum depth to search for .git directories

    Returns:
        List of paths to git repository roots
    """
    repos: list[Path] = []

    def search(path: Path, depth: int) -> None:
        if depth > max_depth:
            return
        if not path.is_dir():
            return
        if path.name in REPO_EXCLUDES:
            return

        # Check if this is a git repo
        if (path / ".git").exists():
            repos.append(path)
            # Still recurse to find nested repos (like work/jren-cm)

        # Recurse into subdirectories
        try:
            for child in path.iterdir():
                if child.is_dir() and child.name not in REPO_EXCLUDES:
                    search(child, depth + 1)
        except PermissionError:
            pass

    search(root_dir, 0)
    return sorted(repos)


@dataclass
class TestGap:
    """Represents a module without a corresponding test file."""

    module_path: str
    expected_test_paths: list[str] = field(default_factory=list)


@dataclass
class ScanResult:
    """Result of scanning a directory for test gaps."""

    directory: str
    total_modules: int
    tested_modules: int
    coverage_percent: float
    missing_tests: list[TestGap] = field(default_factory=list)
    tested: list[str] = field(default_factory=list)

    @property
    def status(self) -> str:
        """Return PASS or FAIL based on coverage."""
        return "PASS" if self.coverage_percent >= 80 else "FAIL"

    def to_dict(self, threshold: int = 80) -> dict:
        """Convert to dictionary for JSON serialization."""
        return {
            "directory": self.directory,
            "total_modules": self.total_modules,
            "tested_modules": self.tested_modules,
            "coverage_percent": round(self.coverage_percent, 1),
            "missing_tests": [
                {"module": g.module_path, "expected_test": g.expected_test_paths[0] if g.expected_test_paths else ""}
                for g in self.missing_tests
            ],
            "status": "PASS" if self.coverage_percent >= threshold else "FAIL",
            "threshold": threshold,
        }


@dataclass
class MultiRepoResult:
    """Result of scanning multiple repositories."""

    root_directory: str
    repos_scanned: int
    repos_with_python: int
    total_modules: int
    total_tested: int
    overall_coverage: float
    results: list[ScanResult] = field(default_factory=list)

    def to_dict(self, threshold: int = 80) -> dict:
        """Convert to dictionary for JSON serialization."""
        return {
            "root_directory": self.root_directory,
            "repos_scanned": self.repos_scanned,
            "repos_with_python": self.repos_with_python,
            "total_modules": self.total_modules,
            "total_tested": self.total_tested,
            "overall_coverage": round(self.overall_coverage, 1),
            "status": "PASS" if self.overall_coverage >= threshold else "FAIL",
            "threshold": threshold,
            "repos": [r.to_dict(threshold) for r in self.results],
        }


def scan_all_repos(
    root_dir: Path,
    include_init: bool = False,
    excludes: list[str] | None = None,
) -> MultiRepoResult:
    """Scan all git repositories under a root directory.

    Args:
        root_dir: Root directory containing repos
        include_init: Include __init__.py files
        excludes: Patterns to exclude

    Returns:
        Aggregated results from all repos
    """
    repos = discover_git_repos(root_dir)
    results: list[ScanResult] = []

    for repo in repos:
        result = scan_directory(
            source_dir=repo,
            include_init=include_init,
            excludes=excludes,
        )
        # Only include repos with Python files
        if result.total_modules > 0:
            results.append(result)

    # Calculate totals
    total_modules = sum(r.total_modules for r in results)
    total_tested = sum(r.tested_modules for r in results)
    coverage = (total_tested / total_modules * 100) if total_modules > 0 else 100.0

    return MultiRepoResult(
        root_directory=str(root_dir),
        repos_scanned=len(repos),
        repos_with_python=len(results),
        total_modules=total_modules,
        total_tested=total_tested,
        overall_coverage=coverage,
        results=sorted(results, key=lambda r: r.coverage_percent),
    )


def find_python_modules(
    directory: Path,
    excludes: list[str] | None = None,
    include_init: bool = False,
) -> list[Path]:
    """Find all Python modules in a directory, excluding test files and other patterns."""
    if excludes is None:
        excludes = DEFAULT_EXCLUDES

    modules: list[Path] = []

    for py_file in directory.rglob("*.py"):
        # Check if file matches any exclude pattern
        rel_path = str(py_file.relative_to(directory))

        # Skip excluded patterns
        skip = False
        for pattern in excludes:
            # Simple glob matching
            pattern_parts = pattern.replace("**", "").replace("*", "").strip("/")
            if pattern_parts and pattern_parts in rel_path:
                skip = True
                break

        if skip:
            continue

        # Skip __init__.py unless explicitly included
        if py_file.name == "__init__.py" and not include_init:
            continue

        # Skip test files
        if py_file.name.startswith("test_") or py_file.name.endswith("_test.py"):
            continue

        # Skip conftest.py
        if py_file.name == "conftest.py":
            continue

        modules.append(py_file)

    return sorted(modules)


def find_test_for_module(
    module_path: Path,
    source_dir: Path,
    test_dirs: list[Path],
) -> Path | None:
    """Find the corresponding test file for a module."""
    module_name = module_path.stem
    test_filename = f"test_{module_name}.py"

    # Get the relative path of the module from source
    try:
        rel_path = module_path.relative_to(source_dir)
    except ValueError:
        rel_path = module_path

    # Check various test file locations
    search_paths: list[Path] = []

    # 1. Co-located test (same directory as module)
    search_paths.append(module_path.parent / test_filename)

    # 2. Sibling tests/ directory relative to module
    search_paths.append(module_path.parent / "tests" / test_filename)
    search_paths.append(module_path.parent / "tests" / "unit" / test_filename)

    # 3. Parent's tests/ directory (for nested modules)
    search_paths.append(module_path.parent.parent / "tests" / test_filename)
    search_paths.append(module_path.parent.parent / "tests" / "unit" / test_filename)

    # 4. Root test directories
    for test_dir in test_dirs:
        # Direct: tests/test_module.py
        search_paths.append(test_dir / test_filename)

        # Unit subdirectory: tests/unit/test_module.py
        search_paths.append(test_dir / "unit" / test_filename)

        # Mirror structure: tests/path/to/test_module.py
        if rel_path.parent != Path("."):
            search_paths.append(test_dir / rel_path.parent / test_filename)
            search_paths.append(test_dir / "unit" / rel_path.parent / test_filename)

    # Check if any test file exists
    for test_path in search_paths:
        if test_path.exists():
            return test_path

    return None


def get_expected_test_paths(
    module_path: Path,
    source_dir: Path,
    test_dirs: list[Path],
) -> list[str]:
    """Get list of expected test file paths for a module."""
    module_name = module_path.stem
    test_filename = f"test_{module_name}.py"

    try:
        rel_path = module_path.relative_to(source_dir)
    except ValueError:
        rel_path = module_path

    paths: list[str] = []
    for test_dir in test_dirs:
        paths.append(str(test_dir / test_filename))
        paths.append(str(test_dir / "unit" / test_filename))
        if rel_path.parent != Path("."):
            paths.append(str(test_dir / rel_path.parent / test_filename))

    return paths[:3]  # Return top 3 suggestions


def scan_directory(
    source_dir: Path,
    test_dirs: list[Path] | None = None,
    include_init: bool = False,
    excludes: list[str] | None = None,
) -> ScanResult:
    """Scan a directory for Python modules and identify test gaps."""
    if test_dirs is None:
        # Default test directories relative to source
        test_dirs = [
            source_dir / "tests",
            source_dir / "test",
            source_dir.parent / "tests",
        ]
        test_dirs = [d for d in test_dirs if d.exists()]
        if not test_dirs:
            test_dirs = [source_dir / "tests"]  # Default even if doesn't exist

    modules = find_python_modules(source_dir, excludes=excludes, include_init=include_init)

    missing: list[TestGap] = []
    tested: list[str] = []

    for module in modules:
        test_file = find_test_for_module(module, source_dir, test_dirs)
        rel_module = str(module.relative_to(source_dir))

        if test_file:
            tested.append(rel_module)
        else:
            expected = get_expected_test_paths(module, source_dir, test_dirs)
            missing.append(TestGap(module_path=rel_module, expected_test_paths=expected))

    total = len(modules)
    tested_count = len(tested)
    coverage = (tested_count / total * 100) if total > 0 else 100.0

    return ScanResult(
        directory=str(source_dir),
        total_modules=total,
        tested_modules=tested_count,
        coverage_percent=coverage,
        missing_tests=missing,
        tested=tested,
    )


def print_console_report(result: ScanResult, threshold: int = 80) -> None:
    """Print human-readable console report."""
    print()
    print("Test Gap Scanner Report")
    print("=" * 50)
    print()
    print(f"Directory: {result.directory}")
    print(f"Total modules: {result.total_modules}")
    print(f"Modules with tests: {result.tested_modules}")
    print(f"Coverage: {result.coverage_percent:.1f}%")
    print()

    if result.missing_tests:
        print(f"MISSING TESTS ({len(result.missing_tests)} modules):")
        for gap in result.missing_tests:
            print(f"  \u274c {gap.module_path}")
            if gap.expected_test_paths:
                print(f"     Expected: {gap.expected_test_paths[0]}")
        print()

    if result.tested:
        print(f"TESTED ({len(result.tested)} modules):")
        for module in result.tested[:10]:  # Show first 10
            print(f"  \u2714 {module}")
        if len(result.tested) > 10:
            print(f"  ... and {len(result.tested) - 10} more")
        print()

    # Status
    status = "PASS" if result.coverage_percent >= threshold else "FAIL"
    status_icon = "\u2714" if status == "PASS" else "\u274c"
    print(f"Status: {status_icon} {status} (threshold: {threshold}%)")
    print()

    if result.missing_tests:
        print("RECOMMENDATIONS:")
        print(f"  1. Create test files for {len(result.missing_tests)} missing modules")
        print("  2. Run: pytest tests/ --cov to verify coverage")
        print(f"  3. Add to CI: --min-coverage {threshold} to enforce threshold")
        print()


def print_multi_repo_report(result: MultiRepoResult, threshold: int = 80) -> None:
    """Print human-readable multi-repo report."""
    print()
    print("Test Gap Scanner - Multi-Repo Report")
    print("=" * 60)
    print()
    print(f"Root: {result.root_directory}")
    print(f"Repos scanned: {result.repos_scanned}")
    print(f"Repos with Python: {result.repos_with_python}")
    print(f"Total modules: {result.total_modules}")
    print(f"Total tested: {result.total_tested}")
    print(f"Overall coverage: {result.overall_coverage:.1f}%")
    print()

    # Table header
    print("COVERAGE BY REPOSITORY:")
    print("-" * 60)
    print(f"{'Repository':<35} {'Modules':>8} {'Tested':>8} {'Coverage':>8}")
    print("-" * 60)

    for r in result.results:
        # Extract repo name from full path
        repo_name = Path(r.directory).name
        parent = Path(r.directory).parent.name
        if parent not in (".", "workspaces"):
            repo_name = f"{parent}/{repo_name}"
        status_icon = "\u2714" if r.coverage_percent >= threshold else "\u274c"
        print(f"{status_icon} {repo_name:<33} {r.total_modules:>8} {r.tested_modules:>8} {r.coverage_percent:>7.1f}%")

    print("-" * 60)
    print()

    # High priority gaps (repos with 0% coverage and many modules)
    zero_coverage = [r for r in result.results if r.coverage_percent == 0 and r.total_modules >= 3]
    if zero_coverage:
        print("HIGH PRIORITY GAPS (0% coverage, 3+ modules):")
        for r in sorted(zero_coverage, key=lambda x: -x.total_modules)[:5]:
            repo_name = Path(r.directory).name
            print(f"  \u274c {repo_name}: {r.total_modules} untested modules")
        print()

    # Status
    status = "PASS" if result.overall_coverage >= threshold else "FAIL"
    status_icon = "\u2714" if status == "PASS" else "\u274c"
    print(f"Status: {status_icon} {status} (threshold: {threshold}%)")
    print()


def main() -> int:
    """CLI entry point."""
    parser = argparse.ArgumentParser(
        description="Scan Python codebases for test gaps (Law 7 enforcement)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s .                           # Scan current directory
  %(prog)s ./src --test-dir ./tests    # Custom test directory
  %(prog)s . --format json             # JSON output for CI
  %(prog)s . --min-coverage 80         # Fail if below threshold
  %(prog)s ~/workspaces --all-repos    # Scan all git repos
        """,
    )
    parser.add_argument(
        "directory",
        type=Path,
        help="Directory to scan for Python modules",
    )
    parser.add_argument(
        "--test-dir",
        type=Path,
        action="append",
        dest="test_dirs",
        help="Directory containing test files (can specify multiple)",
    )
    parser.add_argument(
        "--format",
        choices=["console", "json"],
        default="console",
        help="Output format (default: console)",
    )
    parser.add_argument(
        "--min-coverage",
        type=int,
        default=0,
        metavar="PERCENT",
        help="Minimum coverage threshold (exit 1 if below)",
    )
    parser.add_argument(
        "--include-init",
        action="store_true",
        help="Include __init__.py files in scan",
    )
    parser.add_argument(
        "--exclude",
        action="append",
        dest="excludes",
        help="Additional glob patterns to exclude",
    )
    parser.add_argument(
        "--all-repos",
        action="store_true",
        dest="all_repos",
        help="Scan all git repositories under directory",
    )

    args = parser.parse_args()

    if not args.directory.exists():
        print(f"Error: Directory not found: {args.directory}", file=sys.stderr)
        return 1

    # Build exclude patterns
    excludes = DEFAULT_EXCLUDES.copy()
    if args.excludes:
        excludes.extend(args.excludes)

    threshold = args.min_coverage or 80

    # Multi-repo mode
    if args.all_repos:
        result = scan_all_repos(
            root_dir=args.directory.resolve(),
            include_init=args.include_init,
            excludes=excludes,
        )

        # Output
        if args.format == "json":
            print(json.dumps(result.to_dict(threshold), indent=2))
        else:
            print_multi_repo_report(result, threshold=threshold)

        # Exit code based on threshold
        if args.min_coverage > 0 and result.overall_coverage < args.min_coverage:
            return 1

        return 0

    # Single directory mode
    result = scan_directory(
        source_dir=args.directory.resolve(),
        test_dirs=[d.resolve() for d in args.test_dirs] if args.test_dirs else None,
        include_init=args.include_init,
        excludes=excludes,
    )

    # Output
    if args.format == "json":
        print(json.dumps(result.to_dict(threshold), indent=2))
    else:
        print_console_report(result, threshold=threshold)

    # Exit code based on threshold
    if args.min_coverage > 0 and result.coverage_percent < args.min_coverage:
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
