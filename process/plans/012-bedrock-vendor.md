---
id: 012-bedrock-vendor
date: 2026-05-24
status: draft
brief: 012-bedrock-vendor
design: null
adrs: [0014-bedrock-sigv4-and-event-stream, 0015-three-upstreams-selection]
---

# 012 — Bedrock vendor

## Goal

A second adapter that forwards `POST /v1/messages` to AWS Bedrock with SigV4 auth and event-stream → SSE conversion.

## Phases

### Phase 1 — AWS SDK config

- **Outcome:** `aws-sdk-go-v2` pulled; default credential chain resolves; region from `KETE_AWS_REGION` or `AWS_REGION`.
- **Slice:** `internal/adapter/bedrock/config.go`.
- **Context:** ADR 0014; brief 012 Constraints.
- **Depends-on:** `[]`
- **Done when:** unit test loads creds; missing creds → clear error.

### Phase 2 — Body translation

- **Outcome:** `bedrock.TranslateBody(rawAnthropic) ([]byte, error)` strips Anthropic-only fields Bedrock rejects.
- **Slice:** `internal/adapter/bedrock/body.go`. Only re-marshal in the proxy.
- **Context:** ADR 0014; brief 012 Constraints; `internal/adapter/bedrock/config.go`.
- **Depends-on:** `[]`
- **Done when:** fixture Anthropic body → Bedrock body validates.

### Phase 3 — Model id resolution

- **Outcome:** `ResolveModelID(s) (region, modelID, isInferenceProfile)` for ARNs and bare ids.
- **Slice:** `internal/adapter/bedrock/model.go`.
- **Context:** ADR 0015; brief 012.
- **Depends-on:** `[]`
- **Done when:** unit test covers ARN, inference profile, bare id.

### Phase 4 — SigV4 signed POST

- **Outcome:** Builds `model/<id>/invoke-with-response-stream` URL; signs via SDK signer.
- **Slice:** `internal/adapter/bedrock/sign.go`.
- **Context:** `internal/adapter/bedrock/config.go`, `model.go`.
- **Depends-on:** `[phase-1, phase-3]`
- **Done when:** test asserts `Authorization: AWS4-HMAC-SHA256 …`.

### Phase 5 — Event-stream → SSE converter

- **Outcome:** Reads AWS event-stream frames; emits `event: …\ndata: …\n\n` SSE.
- **Slice:** `internal/adapter/bedrock/eventstream.go`.
- **Context:** ADR 0014.
- **Depends-on:** `[]`
- **Done when:** golden test: known event-stream → expected SSE.

### Phase 6 — Adapter impl

- **Outcome:** `bedrock.Adapter` implements `Adapter.Forward` end-to-end.
- **Slice:** `internal/adapter/bedrock/adapter.go` glues body + sign + POST + eventstream.
- **Context:** `internal/adapter/adapter.go` (plan 002 phase 7); `internal/adapter/bedrock/{body,sign,eventstream}.go`.
- **Depends-on:** `[phase-2, phase-4, phase-5]`
- **Done when:** integration against Bedrock-shaped local stub completes a request.

### Phase 7 — Wire selector

- **Outcome:** Plan 002 phase 8 selector recognises `bedrock` from header, ARN model id, or `KETE_UPSTREAM=bedrock`.
- **Slice:** edits to `internal/proxy/route.go`.
- **Context:** `internal/proxy/route.go`; ADR 0015.
- **Depends-on:** `[phase-6]`
- **Done when:** ARN id forces Bedrock even with env=anthropic.

### Phase 8 — `system_name` normalisation

- **Outcome:** Captured task always uses direct-API model id regardless of upstream.
- **Slice:** `internal/adapter/bedrock/normalize.go` called from capture.
- **Context:** brief 012 Constraints; `internal/proxy/capture.go`.
- **Depends-on:** `[phase-6]`
- **Done when:** Bedrock-routed request → task row has direct-API name.

### Phase 9 — AWS-cred redaction

- **Outcome:** Access keys, signed `Authorization`, session tokens never logged.
- **Slice:** extend `internal/proxy/headers.go` redaction list.
- **Context:** `internal/proxy/headers.go` (plan 002 phase 4).
- **Depends-on:** `[]`
- **Done when:** unit test covers AWS header set.

### Phase 10 — `kete doctor` Bedrock check

- **Outcome:** Doctor adds reachability row when AWS creds + region present.
- **Slice:** edit `cmd/kete/doctor.go`.
- **Context:** `cmd/kete/doctor.go` (plan 001 phases 9–10); `internal/adapter/bedrock/config.go`.
- **Depends-on:** `[phase-1]`
- **Done when:** PASS/FAIL row reflects reality.

### Phase 11 — Doc: `docs/how-to/use-bedrock.md`

- **Outcome:** Setup, env vars, model-id forms.
- **Slice:** new file.
- **Context:** brief 012 Doc impact.
- **Depends-on:** `[]`
- **Done when:** file exists.

### Phase 12 — Doc: `docs/explanation/three-upstreams.md`

- **Outcome:** Comparison table including Bedrock cache caveat.
- **Slice:** new file.
- **Context:** brief 012, brief 013.
- **Depends-on:** `[]`
- **Done when:** file exists.

## Out of scope

- Bedrock for OpenAI models. Guardrails / KBs. Hand-rolled SigV4. Model-id remap. Cross-account roles beyond stock SDK.

## Assumptions

- AWS SDK v2 SigV4 + event-stream APIs stable. Bedrock body shape stays Anthropic-message-shaped minus documented fields.
