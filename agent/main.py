#!/usr/bin/env python3
"""
AgentOps Custom Agent - Main Entry Point

Built with Claude Agent SDK, this agent implements the 12-Factor AgentOps
principles and provides a 4-phase workflow: Research -> Plan -> Implement -> Learn.

Usage:
    # Interactive mode
    python -m agent.main

    # Single task mode
    python -m agent.main "Add user authentication with JWT"

    # With specific working directory
    python -m agent.main --cwd /path/to/project "Your task here"

Requirements:
    pip install claude-agent-sdk
    npm install -g @anthropic-ai/claude-code
    export ANTHROPIC_API_KEY=your_key_here
"""

import sys
import argparse
from pathlib import Path
from typing import Optional

try:
    import anyio
    from claude_agent_sdk import (
        query,
        ClaudeAgentOptions,
        AssistantMessage,
        UserMessage,
        TextBlock,
        ToolUseBlock,
        tool,
        create_sdk_mcp_server,
    )
    SDK_AVAILABLE = True
except ImportError:
    SDK_AVAILABLE = False

from .config import SYSTEM_PROMPT, AGENT_CONFIG
from .tools import create_workflow_tools
from .skills import SkillsLoader, create_skill_tool
from .hooks import create_guardrail_hooks, learnings_tracker


def create_agentops_server(skills_loader: SkillsLoader):
    """
    Create the AgentOps MCP server with all workflow tools.

    This server provides:
    - Workflow phase tools (research, plan, implement, learn)
    - Skill loading tool (JIT context loading)
    - Status and utility tools
    """
    if not SDK_AVAILABLE:
        return None

    # Get workflow tools
    workflow_tools = create_workflow_tools()

    # Get skill loading tool
    skill_tool = create_skill_tool(skills_loader)

    # Create decorated tool functions
    @tool("workflow_research", "Start research phase on a topic", {
        "topic": str,
        "approach": str
    })
    async def workflow_research(args):
        handler = next(t["handler"] for t in workflow_tools if t["name"] == "workflow_research")
        return await handler(args)

    @tool("workflow_plan", "Create implementation plan from research", {
        "research_summary": str,
        "constraints": str
    })
    async def workflow_plan(args):
        handler = next(t["handler"] for t in workflow_tools if t["name"] == "workflow_plan")
        return await handler(args)

    @tool("workflow_implement", "Execute implementation with validation", {
        "plan_summary": str,
        "task_number": int
    })
    async def workflow_implement(args):
        handler = next(t["handler"] for t in workflow_tools if t["name"] == "workflow_implement")
        return await handler(args)

    @tool("workflow_learn", "Extract patterns and learnings", {
        "implementation_summary": str,
        "outcomes": str
    })
    async def workflow_learn(args):
        handler = next(t["handler"] for t in workflow_tools if t["name"] == "workflow_learn")
        return await handler(args)

    @tool("workflow_full", "Execute complete 4-phase workflow", {"topic": str})
    async def workflow_full(args):
        handler = next(t["handler"] for t in workflow_tools if t["name"] == "workflow_full")
        return await handler(args)

    @tool("workflow_status", "Get workflow status", {})
    async def workflow_status(args):
        handler = next(t["handler"] for t in workflow_tools if t["name"] == "workflow_status")
        return await handler(args)

    @tool("load_skill", "Load domain-specific knowledge", {"skill_name": str})
    async def load_skill(args):
        handler = skill_tool["handler"]
        return await handler(args)

    # Create MCP server
    server = create_sdk_mcp_server(
        name="agentops",
        version="1.0.0",
        tools=[
            workflow_research,
            workflow_plan,
            workflow_implement,
            workflow_learn,
            workflow_full,
            workflow_status,
            load_skill,
        ]
    )

    return server


async def run_agent(
    task: str,
    working_dir: Optional[Path] = None,
    max_turns: int = 100,
    interactive: bool = False
) -> None:
    """
    Run the AgentOps agent on a task.

    Args:
        task: The task description or prompt
        working_dir: Working directory for the agent
        max_turns: Maximum conversation turns
        interactive: Whether to run in interactive mode
    """
    if not SDK_AVAILABLE:
        print("Error: claude-agent-sdk not installed")
        print("Install with: pip install claude-agent-sdk")
        print("\nFor now, here's how to use the AgentOps workflow manually:")
        print_manual_instructions()
        return

    # Initialize components
    skills_loader = SkillsLoader()
    agentops_server = create_agentops_server(skills_loader)
    hooks = create_guardrail_hooks()

    # Configure agent options
    options = ClaudeAgentOptions(
        system_prompt=SYSTEM_PROMPT,
        allowed_tools=AGENT_CONFIG["allowed_tools"],
        mcp_servers=[agentops_server] if agentops_server else [],
        hooks=hooks,
        cwd=working_dir or Path.cwd(),
        max_turns=max_turns,
    )

    print("=" * 60)
    print("AgentOps Custom Agent")
    print("12-Factor Operational Framework")
    print("=" * 60)
    print()

    if interactive:
        await run_interactive(options)
    else:
        await run_single_task(task, options)

    # Generate learnings summary
    if learnings_tracker.learnings or learnings_tracker.patterns:
        print("\n" + "=" * 60)
        print("LEARNINGS SUMMARY")
        print("=" * 60)
        print(learnings_tracker.generate_summary())


async def run_single_task(task: str, options: ClaudeAgentOptions) -> None:
    """Run a single task through the agent."""
    print(f"Task: {task}")
    print("-" * 60)
    print()

    async for message in query(prompt=task, options=options):
        if isinstance(message, AssistantMessage):
            for block in message.content:
                if isinstance(block, TextBlock):
                    print(block.text)
                elif isinstance(block, ToolUseBlock):
                    print(f"\n[Tool: {block.name}]")


async def run_interactive(options: ClaudeAgentOptions) -> None:
    """Run in interactive mode."""
    print("Interactive mode. Type 'quit' to exit.")
    print("Available commands: /research, /plan, /implement, /learn, /workflow")
    print("-" * 60)
    print()

    while True:
        try:
            user_input = input("You: ").strip()

            if not user_input:
                continue

            if user_input.lower() in ["quit", "exit", "q"]:
                print("Goodbye!")
                break

            # Handle slash commands
            if user_input.startswith("/"):
                user_input = convert_slash_command(user_input)

            async for message in query(prompt=user_input, options=options):
                if isinstance(message, AssistantMessage):
                    for block in message.content:
                        if isinstance(block, TextBlock):
                            print(f"\nAgent: {block.text}")

            print()

        except KeyboardInterrupt:
            print("\n\nInterrupted. Goodbye!")
            break
        except EOFError:
            print("\n\nGoodbye!")
            break


def convert_slash_command(command: str) -> str:
    """Convert slash commands to tool invocations."""
    parts = command.split(maxsplit=1)
    cmd = parts[0].lower()
    args = parts[1] if len(parts) > 1 else ""

    conversions = {
        "/research": f"Use workflow_research tool with topic: {args or 'the current task'}",
        "/plan": f"Use workflow_plan tool with research_summary: {args or 'previous research'}",
        "/implement": f"Use workflow_implement tool with plan_summary: {args or 'current plan'}",
        "/learn": "Use workflow_learn tool to extract patterns from this session",
        "/workflow": f"Use workflow_full tool with topic: {args or 'the current task'}",
        "/status": "Use workflow_status tool",
        "/skills": "Use load_skill tool with skill_name: list",
    }

    return conversions.get(cmd, command)


def print_manual_instructions() -> None:
    """Print instructions for manual usage without SDK."""
    print("""
MANUAL AGENTOPS WORKFLOW
========================

Without the SDK, you can still use the AgentOps workflow manually
by following these phases:

1. RESEARCH PHASE (20%)
   - Understand the problem
   - Explore the codebase with Glob/Grep
   - Read relevant files
   - Document findings

2. PLAN PHASE (30%)
   - Design the solution
   - Break into tasks
   - Get approval before implementing

3. IMPLEMENT PHASE (40%)
   - Execute tasks in order
   - Validate after each change (test, lint)
   - Commit with learnings

4. LEARN PHASE (10%)
   - Extract patterns
   - Document improvements
   - Update knowledge base

THE FIVE LAWS
=============
1. ALWAYS Extract Learnings
2. ALWAYS Improve Self or System
3. ALWAYS Document Context for Future
4. ALWAYS Validate Before Execute
5. ALWAYS Share Patterns Publicly

To install the SDK:
    pip install claude-agent-sdk
    npm install -g @anthropic-ai/claude-code
    export ANTHROPIC_API_KEY=your_key
""")


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description="AgentOps Custom Agent - 12-Factor Operational Framework"
    )
    parser.add_argument(
        "task",
        nargs="?",
        default="",
        help="Task to execute (omit for interactive mode)"
    )
    parser.add_argument(
        "--cwd",
        type=Path,
        default=None,
        help="Working directory for the agent"
    )
    parser.add_argument(
        "--max-turns",
        type=int,
        default=100,
        help="Maximum conversation turns"
    )
    parser.add_argument(
        "-i", "--interactive",
        action="store_true",
        help="Run in interactive mode"
    )

    args = parser.parse_args()

    # Determine mode
    interactive = args.interactive or not args.task

    # Run agent
    if SDK_AVAILABLE:
        anyio.run(
            run_agent,
            args.task or "Hello! I'm ready to help with the AgentOps workflow.",
            args.cwd,
            args.max_turns,
            interactive
        )
    else:
        print("Claude Agent SDK not installed.")
        print_manual_instructions()


if __name__ == "__main__":
    main()
