# Approved Launcher Layout

Based on the approved vLLM layout of 2026-09-27, with GGUF conventions from
Strata/Unsloth and measured performance guidance from Strata and Ingine.
Adapt the facts for the engine, not the screen structure.

## One Screen

- One header: brand, status, selected `#ID short name, effort`, readiness badge
  such as `NOT YET BOOTED`, and the loaded alias only when relevant. No subtitle,
  count, duplicate heading, separate badge row, or standalone URL box.
- Persistent left picker: one combined `MODEL: Family, 98304, DFLASH (7)`
  heading per group, no blank rows, compact fixed ID chips and clear selection
  highlight. Use a recognizable model alias including generation and size as
  needed; full checkpoint identity goes on the right. Do not renumber profiles.
  Keep effort inline. Omit text-only vision markers; append `V` to enabled
  choices. Associate vision with the correct context in mixed groups:
  `MODEL: Family, 49152 / 32768 vision, MTP (3)`.
- Right: three aligned untitled boxes, one border each, never nested.
- One footer: arrows, number jump, Enter review, 0 status, R refresh, Q quit.
- Dark palette: model orange `#FF9E64`, runtime green `#6EE7A8`, API purple
  `#D68CFF`, brand/API accent cyan `#39D0D8`, values `#E6EDF3`, bright
  `#F8FAFC`, muted `#8192A3`. Commas are grey and followed by one space;
  avoid decorative separators. Keep plain/NO_COLOR behavior and measure
  terminal cell widths without counting ANSI escapes.

## Detail Rows

| Box | Content |
| --- | --- |
| Orange | `Model`: full `author \ repository`. `HF link`: exact repository URL, with a short clickable label at narrow widths. `Base model`: verified lineage, architecture, blocks and native context. `Built from`: distinct immediate parent, if known. `Weights`: file/type and size **on disk**, plus tensor mix where useful. Generic numbered shards are just `safetensors`. GGUF uses the meaningful filename and on-disk size, plus a separate `Quant` row before Weights (quant, bpw and maker). `Draft`: identity, method/count, file/type, size together. `Vision`: enabled/default, with `Projector` filename and size; merge at narrow widths. |
| Green | `Context`: exact limits and slots. `VRAM / RAM`: configured GPU **budget** versus independently measured usage; unknown RAM/KV is `unmeasured`. `Device`: identity and clearly labeled current usage. `Batch / CPU`, `Attention` (backend, state dtype, graphs), `Quant / KV` (GGUF: `KV cache` only), `Runtime` (build, CUDA, capability), and `Performance`. Omit inapplicable rows; MoE may include expert residency. |
| Purple | One copyable `API URL` including `/v1`; `Template` name/version/source/size without repeated provenance; exact served `Model id`; distinct aliases/client IDs only; actual `Sampling` defaults and `Reasoning` state/effort/parsers/tool choice. |

Copy exact context numbers. Do not silently infer client caps from server
context. Where the Windows GGUF launcher has an explicit Workbench policy, show
`Context tokens`, `Max output tokens` and input room: use the server output cap
when present, otherwise the profile policy (32768 or a quarter of a smaller
window). Weights on disk, configured allocation, device totals and per-profile
measured usage are different facts.

## Performance

Keep these rows in the green runtime box when the adapter supplies them.

| Row | Content |
| --- | --- |
| `Memory settings` | Configured expert RAM cap, startup RAM headroom, VRAM reserve, KV precision and GPU-resident positions, and conversation-cache budget. |
| `Bench experts` | Measured expert placement across GPU, host RAM, and SSD from the saved benchmark. |
| `Performance` | Measured decode and prefill rates, time to first token, memory observations, and the benchmark context and sample count. |

Use a green-box `Performance` or `PERF` row.
Example, synthetic: `68 tok/s decode (61-74), 1.4 s cached TTFT, 49152 input, n=5`.
Name the metric: decode is not prefill, cached TTFT is not cold TTFT, and neither
is end-to-end response throughput. When available include prefill rate, context
depth, sample count, draft acceptance and measurement date. Keep the detailed
evidence and matching configuration in local adapter data.

Use logged measurements matching the profile, build, hardware and settings.
Ingine's producer checks the binary hash, context and sample count before
displaying its cached series. On absence or mismatch use `unmeasured`
(Ingine: `Not benchmarked on this build`). A group can end in a measured
`67 tok/s` only when representative of that group and current configuration;
omit the suffix otherwise. Keep the draft detail intact. Never benchmark when
opening the picker. Live status metrics are separate and explicitly labeled.

## Fit And Review

Use 80×25 as the reference layout. Hold the columns steady while moving between
profiles. Size the left pane to its longest heading or choice when there is
enough room to keep the details readable. Keep every choice on one row,
including its ID, effort, and vision marker. Use short variant names when the
heading already identifies the model.
If the label is still too long, shorten it with an ellipsis. Keep full names and
settings in the details. When the list exceeds the available rows, keep the
selected ID and its group visible and show the visible range in the picker.

Show all three detail boxes when they fit. When a profile needs more rows,
keep the same colors in a compact overview with model, quant, HF link,
context, memory settings, benchmark expert placement, performance, API URL,
and served ID when supplied. Use the runtime box width for longer summary
rows. Page Down opens the full values and Page Up returns to the overview
from the first page.
Keep the picker visible beside the details while both panes fit. In a narrower
or shorter terminal, Page Down opens details across the full width and Page Up
returns to the picker from the first page. The footer shows the available
controls and the current detail page.

The review shows the selected ID, exact served ID, context, and effort when
supplied. Status-only review shows the caller's latest status snapshot.

Verify the widest fields and longest vision variants at 80×25, above it, and
in narrower and shorter terminals. Check the bottom of the purple box,
actual comma color, NO_COLOR, exact links, and the cancel and status-only
handoffs. Use synthetic data or saved catalogs for the checks.
