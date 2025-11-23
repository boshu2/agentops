"""
AgentOps Custom Agent - Built with Claude Agent SDK

Converts the AgentOps plugin marketplace into a programmatic custom agent
that implements the 12-Factor AgentOps principles.

Components:
- main.py: Entry point and agent orchestration
- tools.py: Workflow commands as SDK tools
- skills.py: JIT skill loading
- hooks.py: Constitutional guardrails (Five Laws)
- config.py: System prompts and configuration
"""

from .config import SYSTEM_PROMPT, AGENT_CONFIG
from .tools import create_workflow_tools
from .skills import SkillsLoader
from .hooks import create_guardrail_hooks

__version__ = "1.0.0"
__all__ = [
    "SYSTEM_PROMPT",
    "AGENT_CONFIG",
    "create_workflow_tools",
    "SkillsLoader",
    "create_guardrail_hooks",
]
