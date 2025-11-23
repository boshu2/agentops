"""
AgentOps Hooks - Constitutional Guardrails (Factor XI)

Implements the Five Laws through SDK hooks that intercept tool calls
and enforce validation requirements.

Hooks:
- pre_tool: Validates before tool execution (Law 4: Validate Before Execute)
- post_tool: Captures learnings after execution (Law 1: Extract Learnings)
- on_error: Documents failures for improvement (Law 5: Share Patterns)
"""

from typing import Dict, Any, Optional, Callable, Awaitable
from datetime import datetime
import re


# Dangerous patterns that require validation gate
DANGEROUS_PATTERNS = [
    # Destructive file operations
    r"rm\s+-rf\s+/",
    r"rm\s+-rf\s+~",
    r"rm\s+-rf\s+\*",
    r"rm\s+-rf\s+\.",

    # Database destructive operations
    r"DROP\s+TABLE",
    r"DROP\s+DATABASE",
    r"DELETE\s+FROM\s+\w+\s*;",
    r"TRUNCATE\s+TABLE",

    # Kubernetes destructive operations
    r"kubectl\s+delete\s+namespace",
    r"kubectl\s+delete\s+--all",
    r"helm\s+uninstall\s+--all",

    # Git dangerous operations
    r"git\s+push\s+.*--force",
    r"git\s+reset\s+--hard",
    r"--no-verify",
    r"--skip-hooks",

    # Process operations
    r"kill\s+-9",
    r"killall",
    r"pkill",
]

# Patterns that should trigger validation warning
WARNING_PATTERNS = [
    r"sudo\s+",
    r"chmod\s+777",
    r"curl\s+.*\|\s*sh",
    r"wget\s+.*\|\s*sh",
    r"eval\s+",
]


def create_guardrail_hooks() -> Dict[str, Callable[..., Awaitable[Dict[str, Any]]]]:
    """
    Create hooks for constitutional guardrails.

    Returns a dictionary of hook functions that can be passed to
    ClaudeAgentOptions.hooks
    """
    return {
        "preTool": pre_tool_hook,
        "postTool": post_tool_hook,
    }


async def pre_tool_hook(
    input_data: Dict[str, Any],
    tool_use_id: str,
    context: Dict[str, Any]
) -> Dict[str, Any]:
    """
    Pre-tool hook implementing Law 4: ALWAYS Validate Before Execute.

    Intercepts tool calls to:
    - Block dangerous patterns
    - Warn about risky operations
    - Enforce validation requirements
    """
    tool_name = input_data.get("tool_name", "")
    tool_input = input_data.get("tool_input", {})

    # Check Bash commands
    if tool_name == "Bash":
        command = tool_input.get("command", "")
        validation_result = validate_command(command)

        if validation_result["blocked"]:
            return {
                "hookSpecificOutput": {
                    "permissionDecision": "deny",
                    "reason": validation_result["reason"],
                    "suggestion": validation_result.get("suggestion", ""),
                    "law_violated": "Law 4: ALWAYS Validate Before Execute",
                }
            }

        if validation_result["warning"]:
            # Allow but log warning
            return {
                "hookSpecificOutput": {
                    "warning": validation_result["reason"],
                    "proceed": True,
                }
            }

    # Check file writes for dangerous content
    if tool_name in ["Write", "Edit"]:
        content = tool_input.get("content", "") or tool_input.get("new_string", "")
        file_path = tool_input.get("file_path", "")

        # Block writes to sensitive files
        sensitive_paths = ["/etc/", "/usr/", "/bin/", "/sbin/", "~/.ssh/", "~/.aws/"]
        for sensitive in sensitive_paths:
            if file_path.startswith(sensitive):
                return {
                    "hookSpecificOutput": {
                        "permissionDecision": "deny",
                        "reason": f"Cannot write to sensitive path: {sensitive}",
                        "law_violated": "Law 4: ALWAYS Validate Before Execute",
                    }
                }

    # Allow all other operations
    return {}


async def post_tool_hook(
    input_data: Dict[str, Any],
    tool_use_id: str,
    result: Dict[str, Any],
    context: Dict[str, Any]
) -> Dict[str, Any]:
    """
    Post-tool hook implementing Law 1: ALWAYS Extract Learnings.

    Captures information after tool execution for:
    - Pattern extraction
    - Metrics tracking
    - Failure documentation
    """
    tool_name = input_data.get("tool_name", "")
    tool_input = input_data.get("tool_input", {})

    # Track metrics
    metrics = {
        "timestamp": datetime.now().isoformat(),
        "tool": tool_name,
        "success": result.get("success", True),
    }

    # Capture learnings from test runs
    if tool_name == "Bash":
        command = tool_input.get("command", "")
        output = result.get("output", "")

        if any(test_cmd in command for test_cmd in ["pytest", "npm test", "go test"]):
            metrics["type"] = "test_run"
            metrics["passed"] = "PASSED" in output or "passed" in output.lower()
            metrics["failed"] = "FAILED" in output or "failed" in output.lower()

        if any(lint_cmd in command for lint_cmd in ["ruff", "eslint", "golangci-lint", "mypy"]):
            metrics["type"] = "lint_run"
            metrics["clean"] = result.get("exit_code", 0) == 0

    # Return metrics for potential logging
    return {
        "hookSpecificOutput": {
            "metrics": metrics,
            "learning_extracted": True,
        }
    }


def validate_command(command: str) -> Dict[str, Any]:
    """
    Validate a bash command against dangerous patterns.

    Returns validation result with blocked status and reason.
    """
    # Check for blocked patterns
    for pattern in DANGEROUS_PATTERNS:
        if re.search(pattern, command, re.IGNORECASE):
            return {
                "blocked": True,
                "warning": False,
                "reason": f"Dangerous pattern detected: {pattern}",
                "suggestion": "Use validation gate: preview changes, test in safe environment first",
            }

    # Check for warning patterns
    for pattern in WARNING_PATTERNS:
        if re.search(pattern, command, re.IGNORECASE):
            return {
                "blocked": False,
                "warning": True,
                "reason": f"Potentially risky operation: {pattern}",
            }

    return {
        "blocked": False,
        "warning": False,
        "reason": None,
    }


class LearningsTracker:
    """
    Track learnings throughout the workflow for Law 1 compliance.

    Collects patterns, decisions, and outcomes for documentation.
    """

    def __init__(self):
        self.learnings: list[Dict[str, Any]] = []
        self.patterns: list[Dict[str, str]] = []
        self.improvements: list[Dict[str, str]] = []
        self.metrics: Dict[str, Any] = {
            "tools_used": {},
            "tests_run": 0,
            "tests_passed": 0,
            "errors_caught": 0,
            "files_modified": 0,
        }

    def add_learning(
        self,
        context: str,
        solution: str,
        learning: str,
        impact: str
    ) -> None:
        """Add a learning in the required format (Law 3)."""
        self.learnings.append({
            "timestamp": datetime.now().isoformat(),
            "context": context,
            "solution": solution,
            "learning": learning,
            "impact": impact,
        })

    def add_pattern(self, name: str, description: str, example: str) -> None:
        """Add a reusable pattern (Law 5)."""
        self.patterns.append({
            "name": name,
            "description": description,
            "example": example,
        })

    def add_improvement(self, area: str, suggestion: str, impact: str) -> None:
        """Add an improvement opportunity (Law 2)."""
        self.improvements.append({
            "area": area,
            "suggestion": suggestion,
            "impact": impact,
        })

    def track_tool_use(self, tool_name: str) -> None:
        """Track tool usage for metrics."""
        if tool_name not in self.metrics["tools_used"]:
            self.metrics["tools_used"][tool_name] = 0
        self.metrics["tools_used"][tool_name] += 1

    def track_test_result(self, passed: bool) -> None:
        """Track test results."""
        self.metrics["tests_run"] += 1
        if passed:
            self.metrics["tests_passed"] += 1

    def track_error_caught(self) -> None:
        """Track validation errors caught."""
        self.metrics["errors_caught"] += 1

    def track_file_modified(self) -> None:
        """Track file modifications."""
        self.metrics["files_modified"] += 1

    def generate_summary(self) -> str:
        """Generate learning summary for documentation."""
        summary_parts = [
            "# Workflow Learnings Summary",
            "",
            f"**Generated:** {datetime.now().isoformat()}",
            "",
            "## Metrics",
            "",
            f"- Tools used: {self.metrics['tools_used']}",
            f"- Tests run: {self.metrics['tests_run']}",
            f"- Tests passed: {self.metrics['tests_passed']}",
            f"- Errors caught by validation: {self.metrics['errors_caught']}",
            f"- Files modified: {self.metrics['files_modified']}",
            "",
        ]

        if self.learnings:
            summary_parts.extend([
                "## Learnings (Law 3: Document Context)",
                "",
            ])
            for i, learning in enumerate(self.learnings, 1):
                summary_parts.extend([
                    f"### Learning {i}",
                    f"- **Context:** {learning['context']}",
                    f"- **Solution:** {learning['solution']}",
                    f"- **Learning:** {learning['learning']}",
                    f"- **Impact:** {learning['impact']}",
                    "",
                ])

        if self.patterns:
            summary_parts.extend([
                "## Patterns (Law 5: Share Patterns)",
                "",
            ])
            for pattern in self.patterns:
                summary_parts.extend([
                    f"### {pattern['name']}",
                    f"{pattern['description']}",
                    "",
                    "```",
                    pattern['example'],
                    "```",
                    "",
                ])

        if self.improvements:
            summary_parts.extend([
                "## Improvements (Law 2: Improve Self/System)",
                "",
            ])
            for improvement in self.improvements:
                summary_parts.extend([
                    f"- **{improvement['area']}:** {improvement['suggestion']}",
                    f"  - Impact: {improvement['impact']}",
                    "",
                ])

        return "\n".join(summary_parts)


# Global learnings tracker instance
learnings_tracker = LearningsTracker()
