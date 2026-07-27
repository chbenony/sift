# 🔎 sift

An identity-aware gateway in front of Anthropic's Claude API. `sift` authenticates every inbound request against Auth0 before forwarding it upstream, so nothing reaches the model without a verified caller identity.

## ⚙️ How it works

1. A caller sends a request to `sift` with `Authorization: Bearer <token>`.
2. `sift` validates the token as a JWT issued by Auth0: signature verified against Auth0's public keys (JWKS), with issuer, audience, and expiry checked. Unauthenticated or invalid requests get `401` before anything else runs.
3. The validated request body is forwarded as-is to Anthropic's Messages API (`POST /v1/messages`), so fields Anthropic supports beyond the ones `sift` explicitly models (`system`, `tools`, `temperature`, etc.) still pass through correctly.
4. Anthropic's response is relayed back to the caller, preserving the original status code.

## 📋 Requirements

- Go 1.26+
- An [Anthropic API key](https://platform.claude.com/settings/keys)
- An Auth0 tenant with:
  - A custom API defined (this becomes the token audience)
  - A Machine-to-Machine application authorized against that API (client-credentials grant — `sift` expects service-to-service callers, not human logins)

## 🔐 Configuration

Set via environment variables (e.g. in a local `.env`, loaded with `export $(grep -v '^#' .env | xargs)`):

| Variable | Required | Description |
|---|---|---|
| `ANTHROPIC_API_KEY` | yes | Key used for the upstream call to Anthropic |
| `AUTH0_DOMAIN` | yes | Your Auth0 tenant domain, e.g. `your-tenant.us.auth0.com` |
| `AUTH0_AUDIENCE` | yes | The API identifier configured in Auth0 |
| `ANTHROPIC_CLIENT_TIMEOUT` | no | Go duration string (e.g. `90s`) for the upstream HTTP client timeout; defaults to `60s` |

`sift` itself only needs the four variables above to run and validate tokens. The two below are **not used by the server** — they're only needed if you want to generate a test token yourself via the client-credentials grant (see "Getting a test token" below):

| Variable | Required | Description |
|---|---|---|
| `AUTH0_CLIENT_ID` | only for generating test tokens | Client ID of an Auth0 Machine-to-Machine application authorized against your API |
| `AUTH0_CLIENT_SECRET` | only for generating test tokens | Corresponding client secret |

## 🚀 Running locally

```bash
export $(grep -v '^#' .env | xargs)
go run main.go handler.go
```

The server listens on `:9000`.

## 🎟️ Getting a test token

Since auth is client-credentials (machine-to-machine), you can fetch a token directly from Auth0 without a browser login flow:

```bash
TOKEN=$(curl -s -X POST "https://${AUTH0_DOMAIN}/oauth/token" \
  -H "content-type: application/json" \
  -d "{\"client_id\":\"${AUTH0_CLIENT_ID}\",\"client_secret\":\"${AUTH0_CLIENT_SECRET}\",\"audience\":\"${AUTH0_AUDIENCE}\",\"grant_type\":\"client_credentials\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
```

## 💬 Example request

```bash
curl -X POST localhost:9000/ \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"model":"claude-haiku-4-5-20251001","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}'
```

A request without a valid token gets `401` instead of reaching Anthropic.

## 🗂️ Project layout

- `main.go` — entrypoint: reads config, builds the Auth0 validator and HTTP client, wires up the server
- `handler.go` — the gateway logic: reads the request, forwards it to Anthropic, relays the response
- `internal/auth/` — builds the JWT validator (JWKS fetching, signature/issuer/audience checks) against Auth0

## 🤖 CI

- **`lint.yml`** — runs `golangci-lint` on every PR
- **`codex-review.yml`** — runs an LLM-based review on every PR and posts feedback as a comment, skipping PRs from forks (no secret access) and cancelling superseded runs on new pushes

## 🚧 Known limitations

- No `ReadHeaderTimeout`/`ReadTimeout` configured on the HTTP server yet — a slow client can hold a connection open indefinitely (tracked as `TODO(hardening)` in `main.go`)
- No policy layer yet beyond authentication — no per-identity rate limiting, spend caps, or model allowlisting
- No test suite yet; correctness is currently verified by manual end-to-end testing plus lint/build checks in CI
