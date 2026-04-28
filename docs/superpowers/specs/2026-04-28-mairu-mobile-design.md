# mairu-mobile: ACP-over-WebSocket Remote

**Status:** Draft
**Date:** 2026-04-28
**Author:** Eneko Sarasola

## Summary

A React Native (Expo) mobile app that attaches to a coding agent running on a
desktop over a Tailscale tailnet, displays the agent's execution stream, and
lets the user direct the agent by voice or text — including approving/denying
tool-call permission requests remotely. The wire protocol is ACP (Agent
Client Protocol), so the phone is agnostic to which agent harness is running
on the host.

## Goals

- Watch a long-running agent session from the phone in real time.
- Send prompts to the agent by voice (on-device STT) or text.
- Interrupt the active turn (`session/cancel`).
- Approve or deny tool-call permission requests from the phone.
- Be agent-harness-agnostic: works for mairu's own agent, Claude Code,
  Gemini, or anything else that speaks ACP.

## Non-Goals (v1)

- Push notifications while the app is backgrounded (deferred; v1 is
  foreground-attached only).
- Cloud STT or audio streaming to the host.
- File attachment, model switching, or multi-user/multi-tailnet support.
- Web/desktop port of the mobile UI.
- Authentication beyond Tailscale peer identity.

## Architecture

```
┌─────────────┐   Tailnet WSS    ┌──────────────────┐   ACP/stdio   ┌─────────────┐
│ RN (Expo)   │ ◄──────────────► │ mairu acp-bridge │ ◄───────────► │ Agent       │
│ phone       │   ACP frames     │ (daemon, Go)     │   subprocess  │ (CC/Gemini/ │
│             │                  │                  │               │  mairu)     │
└─────────────┘                  └──────────────────┘               └─────────────┘
                                          ▲
                                          │ in-process
                                          ▼
                                  ┌──────────────────┐
                                  │ mairu agent      │ (when target is mairu)
                                  └──────────────────┘
```

The phone speaks ACP exclusively. A new `mairu acp-bridge` daemon is the only
network-facing component. Per session, the bridge either:

1. spawns an external ACP-speaking agent as a subprocess and proxies frames
   between the WS and the subprocess's stdin/stdout (Claude Code, Gemini,
   external mairu CLI, etc.), or
2. attaches in-process to mairu's existing agent runner via the same ACP
   surface already exposed by `mairu/internal/acp/`.

Agent-agnosticism falls out of ACP being the canonical protocol.

## Components

| Component | Path | Role |
|---|---|---|
| `acp-bridge` daemon | `mairu/internal/acpbridge/` | WS server, session registry, ring-buffer for replay, per-session ACP client. |
| HTTP/WS endpoints | embedded in `acp-bridge` | `GET /sessions`, `POST /sessions`, `WS /acp?session=<id>`. |
| CLI command | `mairu acp-bridge` | Starts the daemon (default `:7777`, bind `tailscale0` only). |
| RN app | `mobile/` | Expo + TypeScript app. |
| ACP client lib (RN) | `mobile/src/acp/` | Typed ACP frames, WS transport, reconnect with `last-event-id`, request/response correlation. |
| UI | `mobile/src/ui/` | Timeline view, session picker, voice input, permission modal. |

## Wire Protocol

- WebSocket on `ws://<host>:7777/acp?session=<id>` (or omitted = newest active).
- Each WS text frame is one ACP JSON-RPC message. No additional framing.
- Server→client frames are tagged with a monotonic `event_id` (added as a
  sibling field; ignored by stock ACP clients).
- On reconnect the client sends the `Last-Event-ID` header. The daemon replays
  every buffered event with `event_id > last_event_id` for that session, then
  resumes live streaming. Ring buffer: 500 events or ~5 minutes per session,
  whichever fills first.
- HTTP endpoints:
  - `GET /sessions` → `[{id, agent, project, started_at, last_activity_at, active}]`
  - `POST /sessions` body `{agent: "mairu" | "claude-code" | "gemini" | ...,
    project?: string}` → `{id}`. Spawns or attaches.
  - `DELETE /sessions/:id` → terminates.

## Phone → Agent Control Surface (v1)

The phone is allowed to send these ACP methods:

- `session/prompt` — text prompt (from voice transcript or keyboard).
- `session/cancel` — interrupt the active turn.
- Permission responses to server-initiated `session/request_permission`
  (allow / deny / always-allow-for-session).

Anything else from the phone is rejected by the bridge.

## Permission Request Flow

When the agent calls `session/request_permission`:

1. Bridge fans out to all attached clients for that session (mobile + any
   local TTY).
2. First responder wins; the bridge cancels the prompt on the others.
3. If no client responds within 60s, the bridge falls back to the local TTY
   prompt (or denies, if no TTY is attached). Rationale: an unreachable phone
   must never block the agent indefinitely.
4. The phone shows a modal sheet with tool name, args (pretty-printed JSON,
   collapsible for large payloads), and Allow / Deny / Always-allow buttons.
   Haptic buzz on arrival.

## Authentication

- The daemon binds to the `tailscale0` interface only (configurable). It
  refuses connections from any other interface.
- On WS upgrade, the daemon calls Tailscale's local API (`LocalClient.WhoIs`)
  with the peer address. Connections without a Tailscale identity in the same
  tailnet are rejected.
- No app-level password, token, or PIN in v1. Tailscale device auth is the
  identity boundary.

## Mobile UI (v1)

Single-screen timeline; React Native + Expo + TypeScript.

- **Header:** session picker (dropdown of `GET /sessions` + "+ New session"
  picker for agent type), connection indicator, Stop button (visible during
  active turn).
- **Stream:** scrolling list of events:
  - User messages (right-aligned bubble).
  - Assistant messages (markdown rendered).
  - Tool calls (collapsible card: name + args + result/diff).
  - Thinking blocks (dimmed, collapsed by default).
- **Footer:** text input + mic button. Mic uses `@react-native-voice/voice`
  for on-device STT (push-to-talk: hold to record, release to stop).
  Transcribed text lands in the input field; user reviews and hits Send. No
  audio leaves the phone.
- **Permission modal:** slide-up sheet, Allow / Deny / Always-allow.

State lives in a single store (Zustand or similar). Per-session event arrays
capped at 1000 entries client-side; older entries collapse to a "show
earlier" expander.

## Reconnect Behavior

- Phone retries WS with exponential backoff (1s → 30s, jittered).
- On successful reconnect, sends `Last-Event-ID` and merges replayed frames
  before live ones (de-duped by `event_id`).
- If the session has ended server-side, the daemon returns HTTP 410 on the
  upgrade; the phone surfaces "session ended" and offers the session picker.

## Repository Layout

```
mairu/
  internal/
    acpbridge/         # new
      bridge.go
      session.go
      ringbuffer.go
      ws.go
      tailscale_auth.go
mobile/                # new
  app.json
  package.json
  src/
    acp/
    ui/
    state/
    voice/
```

## Testing

- **Go:** unit tests for ring buffer, session registry, last-event-id replay,
  Tailscale auth gate (mocked `WhoIs`); integration test that spins the
  bridge against an in-process fake ACP agent and a `gorilla/websocket`
  client, asserting full prompt → tool-call → permission → result flow.
- **RN:** Jest unit tests for the ACP client (frame parsing, reconnect, event
  dedup); component tests for timeline rendering with synthetic event
  fixtures. No on-device E2E for v1.

## Risks and Open Questions

1. **ACP frame extension:** adding `event_id` as a sibling field may not
   round-trip cleanly through some ACP libraries. Mitigation: namespace it
   (`x-mairu-event-id`) or carry it as a WS subprotocol-level header instead.
   Decision deferred to implementation; both are cheap.
2. **Subprocess agents:** spawning Claude Code / Gemini per session means
   the bridge inherits responsibility for their auth (API keys, login state).
   v1 reads from the user's existing CLI configs; no key management UI.
3. **Voice quality:** on-device STT struggles with code identifiers and
   jargon. Acceptable for "stop", "approve", "run the tests again" — likely
   bad for dictating function names. Cloud STT is the documented escape
   hatch for v2.
4. **Background operation:** without push notifications, the user must keep
   the app foregrounded to be a useful approval surface. iOS will suspend WS
   within ~30s of backgrounding. Documented limitation; v2 adds APNs/FCM via
   the daemon as push provider.

## Rollout

1. Land `internal/acpbridge` + `mairu acp-bridge` CLI command behind a
   feature gate.
2. Land `mobile/` Expo project, ACP client lib, basic timeline UI against a
   local bridge over loopback.
3. Tailscale auth gate + ring-buffer replay.
4. Voice input.
5. Permission modal + interrupt.
6. Dogfood on personal tailnet; iterate.
