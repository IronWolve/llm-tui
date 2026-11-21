# Catalog And Result Protocol

The picker consumes the **resolved** catalog, not an engine's raw profile
config. [The fixture](examples/catalog.json) is a complete example. Catalog
input is a local UTF-8 JSON file (optional BOM) at `MODEL_UI_CATALOG`, default
`catalog.json` beside the executable. Use trusted input: plain presentation
strings, no ANSI sequences, embedded controls, secrets or executable flags.

| Field | Meaning |
| --- | --- |
| `app.brand` | Short engine badge. |
| `app.online`, `app.loaded` | Optional caller-supplied status snapshot and exact loaded alias; defaults offline with no loaded model. R rereads the file, not the engine. |
| `app.title/subtitle/endpoint/state` | Legacy metadata fields (`title`, `subtitle`, `endpoint`, `state`). None are used to display extra lines or initiate probes. |
| `profiles[].id/name/short_name` | Stable unique ID (1-99 in this renderer), long name and short menu label. Ordered list; keep groups contiguous. |
| `served_id/recommended/group/badge` | Exact served alias, default selection flag, combined header before the optional suffix, and readiness badge. Initial online alias match takes precedence over recommendation. |
| `group_suffix` | Optional final heading text, e.g. `MTP (3)` or a matching measured `67 tok/s`. If absent the renderer uses `details["Draft style"]`; explicitly empty means no suffix. Make group heading data consistent across the group. |
| `missing/notes/variant` | Missing required files or prerequisites (blocks review), extra catalog notes, legacy variant. Use notes for nonblocking advisory information. |
| `details` | Ordered array of two-string rows (not a JSON object). Picker reads `Effort`, `Vision` (starts with `on` for V), `Draft style`, and fallback `Context`, `HF link`, `API URL`. Reuse the same facts in compact sections. |
| `compact_sections` | Exactly three arrays of two-string rows, in orange/green/purple order. Empty row values are omitted. API base and exact served ID go in purple. |

## Adapter Workflow

1. Resolve local profile config, template, model/draft/vision metadata and
   matching measured performance. Record unknown data honestly. Opening the
   picker never fetches model cards, downloads, runs inference or probes memory.
   A real launcher may separately do a cheap authorized health/alias check.
2. Export the same data for machine JSON, plain listing and client config.
   Existing producers: vLLM `catalog.py --format json` plus read-only
   `start.sh --describe N`; Strata/Unsloth `start.ps1 -Json` plus `-List`,
   `-Plain`, `-DryRun`. Ingine's `--list` and `--dry-run N` are plain output,
   not a picker JSON producer. These commands describe those launchers, not
   commands included here.
3. Run the picker with `MODEL_UI_CATALOG` and a fresh empty
   `MODEL_UI_RESULT` file in an adapter-owned temporary directory, separate
   from the input. The screen uses stderr. The confirmation result is one
   decimal profile ID and newline, or `0\n` for status only. Without the
   result-file variable it goes to stdout. Quit/error never writes a result;
   stale files are not cleared for you.
4. Arrow keys (or j/k), Home/End, or digits select; two-digit IDs can be entered
   within 800 ms. Enter opens review, Y/Enter confirms, N/Esc goes back and
   Ctrl-C quits. A direct 0 opens status-only review. R reloads the snapshot.
   In the compact overview, Page Down opens the full details and Page Up
   returns from the first page. Changing profiles restores the overview.
5. Check successful exit and a valid result against the launcher's current
   profile data. No result = no action; 0 = read-only status, never proxy/model
   startup. A nonzero result is a selection, not permission to bypass engine
   preflight or replace a running server. Keep dry-run, explicit automation
   consent, log tailing and exact process ownership in the engine adapter.

Keep the post-launch client block authoritative: API type/base/key requirement
(never the secret), exact model ID, context/output limits, input types,
reasoning/effort, tools, system-message handling and sampling. A profile badge
or successful dry run is not a verified boot.

## Launcher scripts

When wiring the picker into an engine, provide these entry points beside the
engine's start script. Use the names for the launcher platform.

| Linux / WSL | Windows | Responsibility |
| --- | --- | --- |
| `status.sh` | `status.ps1` | Report readiness, the loaded model ID, and the owned process or endpoint. Use the same read-only check for picker option `0`. |
| `log.sh` | `log.ps1` | Show or follow the adapter's local log. Keep logs outside Git and redact credentials. |
| `stop.sh` | `stop.ps1` | Stop the adapter's verified process and report whether it exited. |

Each script resolves project paths from its location. Keep these entry points
with the engine adapter.
