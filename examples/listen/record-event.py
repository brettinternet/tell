#!/usr/bin/env python3
"""Store only the most recently observed Slack event identity locally."""

import json
import os
from pathlib import Path
import sys

MAX_BYTES = 256 * 1024
payload = sys.stdin.buffer.read(MAX_BYTES + 1)
if len(payload) > MAX_BYTES:
    print("event exceeds the 256 KiB command input limit", file=sys.stderr)
    raise SystemExit(1)

try:
    event = json.loads(payload)
except (UnicodeDecodeError, json.JSONDecodeError):
    print("event envelope is invalid JSON", file=sys.stderr)
    raise SystemExit(1)

fields = {key: event.get(key) for key in ("source", "type", "id", "received_at")}
if any(not isinstance(value, str) or not value for value in fields.values()):
    print("event envelope identity is incomplete", file=sys.stderr)
    raise SystemExit(1)

state_dir = Path.home() / ".local" / "state" / "tell"
state_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
state_path = state_dir / "last-listen-event.json"
fd = os.open(state_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW, 0o600)
os.fchmod(fd, 0o600)
with os.fdopen(fd, "w", encoding="utf-8") as state:
    json.dump(fields, state, ensure_ascii=True, separators=(",", ":"))
    state.write("\n")
