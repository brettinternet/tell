# New incident channel → desktop

Create a public `inc-*` channel and get a desktop notification.

Follow the [Slack setup](../README.md#slack-setup). Subscribe to
[`channel_created`](https://docs.slack.dev/reference/events/channel_created/)
with `channels:read`.

[config.yaml](config.yaml) matches channel creation, not messages or renames.

```yaml
match:
  event.channel.name: {prefix: "inc-"}
```

`inc-*` means a prefix here, not a glob.

Run from the Tell checkout with a desktop session available.

```sh
tell --config examples/listen/incident-channel/config.yaml config check
tell --config examples/listen/incident-channel/config.yaml listen --dry-run slack
# Create inc-demo, then stop with Ctrl-C.
tell --config examples/listen/incident-channel/config.yaml listen slack
# Create inc-demo-live.
```

| Slack action | Dry-run | Dispatch |
| --- | --- | --- |
| Create public `inc-demo` | Matching route receipt | Desktop notification |
| Create public `team-demo` | No match | No notification |
| Post in an existing `inc-*` channel | No matching creation event | No notification |

Use a fresh channel name for each creation.

## Also inform an agent

Append this route to the same `listen.routes` list. Replace `triage` with a
participating hcom recipient.

```yaml
- name: incident-agent
  source: slack
  type: channel_created
  match:
    event.channel.name: {prefix: "inc-"}
  send:
    via: hcom
    to: triage
    intent: inform
    message: "New incident channel ${data.event.channel.id}; Slack event ${id}. Retrieve context with your own Slack tools."
```

Both routes match independently. Delivery order is not guaranteed; one failure
does not block the other. Tell does not fetch channel history.
