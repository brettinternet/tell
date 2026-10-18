# Slack app mention → agent

Mention the Slack app in one channel to inform a local agent.

Follow the [Slack setup](../README.md#slack-setup). Subscribe to
[`app_mention`](https://docs.slack.dev/reference/events/app_mention/)
with `app_mentions:read`, then invite the app to the channel.

In [config.yaml](config.yaml), replace `C0123456789` with the channel ID
and `triage` with a participating hcom recipient. Copy the channel ID from
Slack's channel details.

```sh
tell --config examples/listen/mention-agent/config.yaml config check
tell --config examples/listen/mention-agent/config.yaml listen --dry-run slack
# Mention the Slack app in the selected channel, then stop with Ctrl-C.
tell --config examples/listen/mention-agent/config.yaml listen slack
# Send a new mention.
```

| Slack action | Result |
| --- | --- |
| Mention the app in the selected channel | Inform `triage` |
| Mention it in another channel the app has joined | No match |
| Send an ordinary message without a mention | No matching mention event |
| DM the app | Not an `app_mention` event |

The agent receives channel/message IDs, not message text. Tell adds external
event provenance and an untrusted-content warning. The agent needs its own
Slack tools and access to retrieve context; Tell does not reply in Slack.
`inform` is not a prompt-injection sandbox.
