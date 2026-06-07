#!/usr/bin/env python3
"""Format a triangulation prompt for copy-paste to other models.

Usage:
    ./format-prompt.py idea "Idea 1" "Idea 2" "Idea 3"
    ./format-prompt.py code < code.py
    ./format-prompt.py arch "Option A desc" "Option B desc"

Writes formatted prompt to stdout (easy to pipe to clipboard).
"""

import sys
from textwrap import dedent

TEMPLATES = {
    "idea": dedent("""
        # COPY TO [Codex/Gemini/Grok]:

        Evaluate these ideas. Score each 1-10 on:
        - **Quality**: Novel and valuable?
        - **Utility**: Solves real problem?
        - **Feasibility**: Effort vs payoff?
        - **Risk**: What could go wrong?

        Ideas:
        {ideas}

        For each:
        1. Scores (Q/U/F/R out of 10)
        2. One-sentence rationale
        3. Final rank best to worst

        Be critical. Don't just agree.
    """).strip(),

    "code": dedent("""
        # COPY TO [Codex/Gemini/Grok]:

        Review this code for bugs, security issues, improvements:

        ```
        {code}
        ```

        Categorize findings:
        1. **Critical** (must fix): Security, data corruption
        2. **Important** (should fix): Logic, edge cases
        3. **Suggestions** (nice to have): Style, minor opts

        For each: location, problem, fix.
        Overall: X/10

        Be thorough. Check edge cases, injection, error handling.
    """).strip(),

    "arch": dedent("""
        # COPY TO [Codex/Gemini/Grok]:

        Choosing between options for [PROBLEM]:

        {options}

        Evaluate each on:
        - Complexity (simple ↔ complex)
        - Maintainability (easy ↔ hard)
        - Performance (fast ↔ slow)
        - Scalability

        Recommend ONE with reasoning. Be opinionated.
    """).strip(),

    "debug": dedent("""
        # COPY TO [Codex/Gemini/Grok]:

        Help debug:

        **Symptom:** {symptom}
        **Expected:** {expected}

        **Code:**
        ```
        {code}
        ```

        Provide:
        1. Top 3 likely causes (ranked)
        2. Diagnostic steps for each
        3. Quick fix vs proper fix
    """).strip(),
}


def format_idea(args: list[str]) -> str:
    ideas = "\n".join(f"{i+1}. {idea}" for i, idea in enumerate(args))
    return TEMPLATES["idea"].format(ideas=ideas)


def format_code(args: list[str]) -> str:
    if args:
        code = " ".join(args)
    else:
        code = sys.stdin.read()
    return TEMPLATES["code"].format(code=code)


def format_arch(args: list[str]) -> str:
    options = "\n\n".join(
        f"**Option {chr(65+i)}:**\n{opt}" for i, opt in enumerate(args)
    )
    return TEMPLATES["arch"].format(options=options)


def format_debug(args: list[str]) -> str:
    if len(args) < 2:
        raise ValueError("debug mode requires at least SYMPTOM and EXPECTED text")

    symptom = args[0]
    expected = args[1]
    if len(args) > 2:
        code = " ".join(args[2:])
    else:
        code = sys.stdin.read()

    return TEMPLATES["debug"].format(
        symptom=symptom,
        expected=expected,
        code=code,
    )


def main():
    if len(sys.argv) < 2:
        print("Usage: format-prompt.py <type> [args...]")
        print("Types: idea, code, arch, debug")
        sys.exit(1)

    prompt_type = sys.argv[1]
    args = sys.argv[2:]

    formatters = {
        "idea": format_idea,
        "code": format_code,
        "arch": format_arch,
        "debug": format_debug,
    }

    if prompt_type not in formatters:
        print(f"Unknown type: {prompt_type}")
        print(f"Available: {', '.join(formatters.keys())}")
        sys.exit(1)

    try:
        result = formatters[prompt_type](args)
    except ValueError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        sys.exit(1)

    print(result)

    # Hint for clipboard
    print("\n# Tip: Pipe to clipboard with: | pbcopy (mac) or | xclip (linux)", file=sys.stderr)


if __name__ == "__main__":
    main()
