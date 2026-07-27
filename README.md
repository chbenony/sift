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

Set via environment variables (e.g. in a local `.env`, loaded with `set -a; source .env; set +a`):

| Variable | Required | Description |
|---|---|---|
| `ANTHROPIC_API_KEY` | yes | Key used for the upstream call to Anthropic |
| `AUTH0_DOMAIN` | yes | Your Auth0 tenant domain, e.g. `your-tenant.us.auth0.com` |
| `AUTH0_AUDIENCE` | yes | The API identifier configured in Auth0 |
| `ANTHROPIC_CLIENT_TIMEOUT` | no | Go duration string (e.g. `90s`) for the upstream HTTP client timeout; defaults to `60s` |

`sift` itself reads the four variables above to run and validate tokens (only the first three are actually required — the timeout has a default). The two below are **not used by the server** — they're only needed if you want to generate a test token yourself via the client-credentials grant (see "Getting a test token" below):

| Variable | Required | Description |
|---|---|---|
| `AUTH0_CLIENT_ID` | only for generating test tokens | Client ID of an Auth0 Machine-to-Machine application authorized against your API |
| `AUTH0_CLIENT_SECRET` | only for generating test tokens | Corresponding client secret |

## 🚀 Running locally

```bash
set -a; source .env; set +a
go run main.go handler.go
```

(`source`ing the file directly avoids spawning any intermediate process with secret values in its argv — unlike `export $(... | xargs)`, which passes every value through `xargs`' own command-line arguments and can also mangle values containing spaces or quotes.)

The server listens on `:9000`.

## 🎟️ Getting a test token

Since auth is client-credentials (machine-to-machine), you can fetch a token directly from Auth0 without a browser login flow:

```bash
set -o pipefail
TOKEN=$(jq -n '{
    client_id: env.AUTH0_CLIENT_ID,
    client_secret: env.AUTH0_CLIENT_SECRET,
    audience: env.AUTH0_AUDIENCE,
    grant_type: "client_credentials"
  }' \
  | curl -s --fail-with-body -X POST "https://${AUTH0_DOMAIN}/oauth/token" \
      -H "content-type: application/json" \
      --data-binary @- \
  | jq -er '.access_token')
```

(Uses `jq`'s `env` accessor and `curl --data-binary @-` rather than inlining secrets as literal command-line arguments — arguments passed directly on a command line are visible to other local users via process listings like `ps aux`, whereas this keeps the secret values out of any process's argv. It also avoids manually interpolating untrusted values into a JSON string, which can produce invalid JSON if a value contains a quote or backslash. `curl --fail-with-body` and `jq -e` make this command fail loudly — instead of silently setting `TOKEN` to the string `null` — if Auth0 returns an error instead of a token, e.g. if the client isn't authorized for the API. `set -o pipefail` ensures the overall exit status reflects a failure anywhere in the pipeline — e.g. a network/TLS failure in `curl` — rather than only the last command's status.)

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
