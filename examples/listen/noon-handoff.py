#!/usr/bin/env python3
"""Queue the Tell envelope as Noon data, preserving bounded direct-child exit."""

import hashlib
import os
import re
import sys

KEY = re.compile(r"[A-Za-z0-9._:-]{1,200}\Z")
source = os.environ.get("TELL_EVENT_SOURCE", "")
event_id = os.environ.get("TELL_EVENT_ID", "")
if not source or not event_id:
    print("Tell event identity is missing", file=sys.stderr)
    raise SystemExit(1)

qualified_id = f"{source}:{event_id}"
if KEY.fullmatch(qualified_id):
    key = qualified_id
else:
    key = "tell:" + hashlib.sha256(qualified_id.encode("utf-8")).hexdigest()

# Tell caps the encoded envelope at 256 KiB; Noon independently validates the
# same --data limit. Pass stdin unchanged and let Noon reject, never truncate.
try:
    os.execvp("noon", ["noon", "run", "incident-kickoff", "--detach", "--key", key, "--data", "-"])
except OSError:
    print("Noon handoff could not start", file=sys.stderr)
    raise SystemExit(1)
