# Local LLM summary: why did it fail?

"Build failed" makes you open a terminal to find out why.
[summarize-failure.sh](summarize-failure.sh) asks a local model for a
one-sentence diagnosis and puts it in the notification, with the full log path.

```sh
examples/local-llm-summary/summarize-failure.sh docker build -t api .
examples/local-llm-summary/summarize-failure.sh bun run e2e
```

```text
Failed: docker
The build fails because package-lock.json is out of sync with package.json, so npm ci aborts.
Exit 1. Log: /Users/me/.local/state/tell-logs/docker.Xa81Qz
```

## Setup

```sh
brew install ollama   # or see ollama.com
ollama pull llama3.2  # small and fast; set OLLAMA_MODEL to use another
```

Any CLI that reads stdin works; swap the `ollama run` line for
`llm -m <model>` or `mlx_lm.generate`.

## Notes

- The log goes only to the local model. Only the summary goes to your
  route. Prefer a local route (desktop, Herdr) because the summary derives
  from logs. Tell still redacts detected secrets.
- The summary is a guess. The log path is the source of truth.
- Logs accumulate in `~/.local/state/tell-logs/`; prune them as you like.
- Large models can take longer to load than the notification is worth.
  Pick a small one.
