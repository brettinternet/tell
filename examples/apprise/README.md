# Apprise: phone, Slack, Discord, and 100+ more

[Apprise](https://github.com/caronc/apprise) holds destinations and
credentials. Tell passes title, body, type, and one tag. Use tags to choose an
audience per message without putting URLs anywhere else.

## Setup

```sh
pipx install apprise        # or: uv tool install apprise
mkdir -p ~/.config/apprise
cp examples/apprise/apprise.yaml ~/.config/apprise/apprise.yaml
chmod 600 ~/.config/apprise/apprise.yaml   # then replace the placeholders
cp examples/apprise/config.yaml ~/.config/tell/config.yaml
tell doctor                 # dry run: config loads and the tag matches ≥1 service
```

| File | Owns |
| --- | --- |
| [apprise.yaml](apprise.yaml) | destinations, tokens, tags (`phone`, `team`, `urgent`, `digest`) |
| [config.yaml](config.yaml) | Tell route: Apprise, default tag `phone`, Markdown bodies |

## Choosing the audience

```sh
tell notify "Model download finished"                         # default tag: phone
tell notify --apprise-tag team --type success "Staging deployed: v1.8.2"
tell notify --apprise-tag urgent --type failure "Prod error rate above 5%"   # phone + Slack
tell notify --apprise-tag digest - < weekly-report.md
```

A shared `tell.yaml` may set a project's default tag, because tags pick an
audience within your destinations rather than a destination:

```yaml
# tell.yaml (committed)
version: 1
apprise:
  tag: team
  format: markdown
```

Each person's Apprise config decides what `team` means for them. A repo
cannot choose `apprise.config`, so it cannot redirect your messages.

## Outcomes

| Apprise result | Tell outcome | Do |
| --- | --- | --- |
| all delivered | `accepted` (0) | nothing |
| no service has the tag | `not_sent/no_match` (3) | fix the tag; safe to resend |
| some delivered | `failed/partial` (4) | check before resending |
| timed out | `uncertain` (5) | check before resending |

## Notes

- Without a tag, Apprise notifies only untagged services.
- Third-party services receive the body. Send pointers (run IDs, URLs), not
  logs. Tell redacts detected secrets, but detection is best-effort.
- Native `APPRISE_*` environment variables pass through; CI can use
  `APPRISE_URLS` instead of a file (see [github-actions](../github-actions/README.md)).
