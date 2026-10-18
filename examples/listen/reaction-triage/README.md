# Eyes reaction → triage agent

Add 👀 to a message in one channel to flag it for review.

Follow the [Slack setup](../README.md#slack-setup). Subscribe to
[`reaction_added`](https://docs.slack.dev/reference/events/reaction_added/)
with `reactions:read`, then invite the app to the channel.

In [config.yaml](config.yaml), replace `C0123456789` with the channel ID
and `triage` with a participating hcom recipient.

```yaml
match:
  event.reaction: eyes
  event.item.type: message
  event.item.channel: C0123456789
```

All three fields must match. Emoji names omit the surrounding colons.

```sh
tell --config examples/listen/reaction-triage/config.yaml config check
tell --config examples/listen/reaction-triage/config.yaml listen --dry-run slack
# Add an eyes reaction, then stop with Ctrl-C.
tell --config examples/listen/reaction-triage/config.yaml listen slack
# React to another message.
```

| Slack action | Result |
| --- | --- |
| Add 👀 to a message in the selected channel | Inform `triage` with channel/message IDs |
| Add 👍 instead | No match |
| Add 👀 in another channel the app has joined | No match |
| Remove 👀 | No matching added-reaction event |

A reaction flags work; it does not authorize changes. This route accepts any
reacting user in the selected channel. For one user, also match
`event.user: U0123456789`.

Separate reactions on the same message can produce separate informs. Tell
deduplicates event IDs, not messages or review requests. The agent must apply
its own review policy and fetch context with its own Slack tools.
