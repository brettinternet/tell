# Eval regression: catch prompt and model drift

You tweak a system prompt, swap a model, or change chunking, and quality
quietly drops. [eval-gate.sh](eval-gate.sh) runs your eval, compares the score
with a committed baseline, and on a regression tells you and asks an agent to
find the cause.

```sh
examples/eval-regression/eval-gate.sh evals/baseline.json uv run python evals/run.py --suite smoke
```

```text
Eval regression
Score 0.74 vs baseline 0.82 at 9b1d7aa. Results: /Users/me/.local/state/tell-evals/9b1d7aa-20261016T091500.txt
```

The agent receives a request with the commit range to inspect:

```text
Eval score fell from 0.82 (commit 3f9c2e1) to 0.74 at 9b1d7aa.
Results: /Users/me/.local/state/tell-evals/9b1d7aa-20261016T091500.txt
Compare prompt, model, and retrieval changes in git log 3f9c2e1..9b1d7aa and report the likely cause.
Do not edit evals/baseline.json.
```

## Contract

Your eval command prints `{"score": N}` as its last stdout line; anything
before it is kept in the results file. Adapt any harness with a final
`jq -c '{score: ...}'` over its output. The baseline is a committed file:

```json
{ "score": 0.82, "commit": "3f9c2e1" }
```

`EVAL_TOLERANCE` (default `0.02`) absorbs run-to-run noise.

## Run it

- Locally, after prompt edits: `task eval` wrapping this script.
- Nightly against `main` with [Noon](../noon/README.md), launchd, or cron, so
  model-provider changes surface even when your code did not change.
- In [test-watch-agent](../test-watch-agent/README.md) style, if your smoke
  suite is fast enough.

Configure `send.via`/`send.to` for the investigating agent, as in
[agent-handoff](../agent-handoff/README.md). Without them, `tell send` returns
`not_sent` and you still get the notification.
