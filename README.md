# 🔎 sift

An identity-aware, usage-metered gateway in front of Anthropic's Claude API. `sift` authenticates every inbound request against Auth0, forwards it to Claude, and reports the token usage to Stripe so each caller can be billed for what they consume. It ships with a React web client for trying it out.

## ✨ What sift does

- **Authenticates every request.** Callers present an Auth0-issued JWT. Nothing reaches the model without a verified identity.
- **Proxies to Claude.** Requests are forwarded to Anthropic's Messages API and the response is relayed back.
- **Meters usage per customer.** After each successful call, input and output token counts are recorded as Stripe billing meter events against the caller's Stripe customer.
- **Exposes two APIs.** A REST-style passthrough at `/` and a GraphQL API at `/graphql` (send a message, read your total usage).
- **Includes a web client.** A React app in `web/` handles Auth0 login and talks to the GraphQL API.

## 🧰 Technologies

| Area | Technology |
|---|---|
| Backend language | Go (see `go.mod` for the version) |
| API | REST passthrough (`net/http`) and GraphQL via [gqlgen](https://gqlgen.com/) |
| Authentication | [Auth0](https://auth0.com/) JWTs (RS256, JWKS) validated with `go-jwt-middleware` |
| LLM | [Anthropic Claude](https://docs.anthropic.com/) Messages API |
| Billing / metering | [Stripe](https://stripe.com/) Billing meters (`stripe-go`) |
| Frontend | React 19, Vite, `@auth0/auth0-react`, ESLint |
| Testing | Go `testing`, `go.uber.org/mock` (mockgen) |
| CI | GitHub Actions: `golangci-lint`, `go test -race`, LLM-assisted PR review, `@claude` mentions |

## ⚙️ How it works

```
Browser (React)  ──GraphQL──▶ ┐
                              ├─▶ sift (Go, :9000) ──▶ Anthropic Messages API
API client ─────REST (/)────▶ ┘        │  ▲
                                       │  └─ Auth0 JWKS (token validation)
                                       └──▶ Stripe meter events (usage billing)
```

1. A caller sends a request with `Authorization: Bearer <token>`.
2. `sift` validates the JWT: signature against Auth0's JWKS, plus issuer, audience, and expiry. The token must also carry the `https://sift.dev/stripe_customer_id` custom claim, which identifies who gets billed. Invalid or missing tokens get `401`.
3. The request is forwarded to Anthropic (`POST /v1/messages`).
4. The response goes back to the caller. For successful calls, `sift` then records input and output token usage to Stripe in the background, keyed by the Anthropic message ID so retries aren't double-counted.
5. On shutdown (`SIGINT`/`SIGTERM`), the server drains in-flight requests and waits up to 10s for pending usage reports.

### Endpoints

| Endpoint | Description |
|---|---|
| `POST /` | Passthrough to the Anthropic Messages API. The body is forwarded as-is, so fields `sift` doesn't model (`system`, `tools`, `temperature`, etc.) still work. The upstream status code is preserved. Bodies are capped at 1 MiB, and `stream: true` is rejected with `400`. |
| `POST /graphql` | GraphQL API (JWT required, CORS allowed for `http://localhost:5173`). |

### GraphQL API

Schema: `graph/schema.graphqls`

```graphql
type Query    { usage: Usage! }                               # total tokens for the calling customer
type Mutation { sendMessage(input: SendMessageInput!): Message! }

input SendMessageInput { model: String!, content: String! }
type Message { id: ID!, content: String!, stopReason: String!, usage: Usage! }
type Usage   { inputTokens: Int64!, outputTokens: Int64! }
```

- `sendMessage` sends a single user message (with `max_tokens` fixed at 1024) and returns the reply text.
- `usage` reads the caller's cumulative input and output tokens back from Stripe's meter summaries.
- Query complexity is capped at 100. WebSocket transport is deliberately disabled, because a socket could outlive its JWT.

## 📋 Requirements

- Go (version in `go.mod`)
- Node.js and npm, to run the web client
- An [Anthropic API key](https://platform.claude.com/settings/keys)
- An Auth0 tenant with:
  - A custom API (its identifier becomes the token audience)
  - A Machine-to-Machine application authorized against that API, for service callers
  - A Single Page Application, for the web client
  - An Action or rule that adds the `https://sift.dev/stripe_customer_id` claim to access tokens
- A Stripe account with two billing meters, one for `anthropic_input_tokens` and one for `anthropic_output_tokens`

## 🔐 Configuration

### Backend

Set these as environment variables (e.g. in a local `.env`, loaded with `set -a; source .env; set +a`):

| Variable | Required | Description |
|---|---|---|
| `ANTHROPIC_API_KEY` | yes | Key used for the upstream call to Anthropic |
| `AUTH0_DOMAIN` | yes | Auth0 tenant domain, e.g. `your-tenant.us.auth0.com` |
| `AUTH0_AUDIENCE` | yes | The API identifier configured in Auth0 |
| `STRIPE_API_KEY` | yes | Stripe secret key used to report and read usage |
| `STRIPE_INPUT_TOKENS_METER_ID` | yes | ID of the Stripe meter for input tokens |
| `STRIPE_OUTPUT_TOKENS_METER_ID` | yes | ID of the Stripe meter for output tokens |
| `ANTHROPIC_CLIENT_TIMEOUT` | no | Go duration (e.g. `90s`) for the upstream HTTP client; defaults to `60s` |

These two are **not used by the server**. You only need them to generate a test token via the client-credentials grant (see below):

| Variable | Description |
|---|---|
| `AUTH0_CLIENT_ID` | Client ID of an Auth0 M2M application authorized against your API |
| `AUTH0_CLIENT_SECRET` | Corresponding client secret |

### Frontend (`web/`)

Create `web/.env.local` with:

| Variable | Description |
|---|---|
| `VITE_AUTH0_DOMAIN` | Auth0 tenant domain |
| `VITE_AUTH0_CLIENT_ID` | Client ID of the Auth0 Single Page Application |
| `VITE_AUTH0_AUDIENCE` | The same API identifier as `AUTH0_AUDIENCE` |

In the Auth0 SPA settings, add `http://localhost:5173` to the allowed callback, logout, and web origin URLs.

## 🚀 Running locally

Backend (listens on `:9000`):

```bash
set -a; source .env; set +a
go run .
```

(`source`ing the file avoids passing secret values through another process's argv, unlike `export $(... | xargs)`, which also mangles values containing spaces or quotes.)

Frontend (serves on `http://localhost:5173`):

```bash
cd web
npm install
npm run dev
```

The client logs in through Auth0, then lets you fetch your token usage and send a message to Claude through the GraphQL API. It calls the backend at `http://localhost:9000/graphql`.

## 🎟️ Getting a test token

For calling the REST or GraphQL endpoints directly, you can fetch a token with the client-credentials grant, without a browser login:

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

(Secrets are passed through `jq`'s `env` and `curl --data-binary @-` so they stay out of process listings. `--fail-with-body`, `jq -e`, and `pipefail` make the command fail loudly instead of setting `TOKEN` to `null`.) The token must include the `stripe_customer_id` claim, or `sift` rejects it.

## 💬 Example requests

REST passthrough:

```bash
curl -X POST localhost:9000/ \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"model":"claude-haiku-4-5-20251001","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}'
```

GraphQL:

```bash
curl -X POST localhost:9000/graphql \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"query":"mutation($i: SendMessageInput!){ sendMessage(input:$i){ content usage{ inputTokens outputTokens } } }","variables":{"i":{"model":"claude-haiku-4-5-20251001","content":"Hello"}}}'

curl -X POST localhost:9000/graphql \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ usage { inputTokens outputTokens } }"}'
```

A request without a valid token gets `401` instead of reaching Anthropic.

## 🧪 Testing and code generation

```bash
go test -race ./...        # backend tests
cd web && npm run lint     # frontend lint
```

After editing `graph/schema.graphqls`, regenerate the GraphQL code with `go tool gqlgen generate` (gqlgen is declared as a `tool` in `go.mod`; config is in `gqlgen.yml`). The Stripe mocks are generated by `mockgen` via the `go:generate` directive in `internal/billing/stripe.go`.

## 🗂️ Project layout

- `main.go`: entrypoint. Reads config, builds the Auth0 validator, Anthropic client, and Stripe reporter, wires up routes and middleware (JWT, CORS, body limit, GraphQL limits), and handles graceful shutdown
- `handler.go`: REST passthrough handler for `POST /`
- `graph/`: GraphQL schema (`schema.graphqls`), resolvers, and generated gqlgen code
- `internal/auth/`: JWT validator (JWKS, signature/issuer/audience checks) and the custom Stripe customer claim
- `internal/anthropic/`: Anthropic Messages API client
- `internal/billing/`: Stripe usage reporting (`Reporter`) and usage reading (`UsageReader`)
- `web/`: React + Vite frontend
- `.github/workflows/`: CI

## 🤖 CI

- **`lint.yml`**: runs `golangci-lint` on every PR
- **`test.yml`**: runs `go test -race ./...` on every PR
- **`codex-review.yml`**: LLM-based review on every PR, posted as a comment. Skips fork PRs (no secret access) and cancels superseded runs
- **`claude.yml`**: responds to `@claude` mentions on issues and PRs

## 🚧 Known limitations

- No `ReadHeaderTimeout`/`ReadTimeout` on the HTTP server yet, so a slow client can hold a connection open indefinitely (tracked as `TODO(hardening)` in `main.go`)
- No policy layer beyond authentication: no per-identity rate limiting, spend caps, or model allowlisting
- Streaming responses are not supported
- The CORS origin and the web client's API URL are hardcoded for local development (`http://localhost:5173` and `http://localhost:9000`)
- The GraphQL `sendMessage` mutation is single-turn with a fixed `max_tokens` of 1024
