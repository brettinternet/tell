## Notifications (Tell MCP)

Use the `tell` MCP tools to reach the user and other agents.

- Call `notify` when you finish a task that took more than a few minutes,
  or when you are blocked on a decision only the user can make. Use one line:
  the outcome or the question, plus a pointer (branch, PR URL, file path).
  Use `type: failure` when blocked or failed, `success` when done.
- After notifying that you are blocked, stop and wait. Do not keep working
  around the question.
- Never include secrets, environment values, or raw logs and diffs in a message.
- Call `send` only to hand work to the configured reviewer agent. Use
  `intent: request` for work you need done and `inform` for status. Include
  the branch and commit SHA; the reviewer reads the code itself.
- `accepted` means delivered to the route, not read. Do not resend an
  accepted, failed, or uncertain message.
