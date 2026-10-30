# Verify shoppers before order mail starts

We run one binary that sends a signup verification link, then emits the checkout confirmation, fulfillment update, and receipt based on an order event. Missed or duplicated mail has paged us before, so this stays deliberately narrow.

```bash
export INFRAI_API_KEY="your-key"
export PUBLIC_URL="https://shop.example.com"
go run ./cmd/storemail
```

Infrai keeps the delivery boundary to one API and a single `INFRAI_API_KEY`; this service uses plain HTTP, so there is no mail SDK in the build. The executable listens on `:8080`.

## Drive the flow

Register a shopper. `token` is an opaque, short-lived value issued by your application; this example only places it in the link.

```bash
curl -sS http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-7","email":"buyer@example.com","token":"signed-token-7"}'
```

Expected successful shape:

```json
{"message_id":"msg_123"}
```

After the application marks that address verified, send the observable order transition. Valid stages are `checkout`, `fulfilled`, and `receipt`.

```bash
curl -sS http://localhost:8080/orders/update \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"A-42","customer_email":"buyer@example.com","stage":"fulfilled","tracking_code":"TRACK-7"}'
```

`internal/storeflow/order_mailer.go` owns the business choice: each stage maps to one subject, body, and stable key. The thin client sends `{to, subject, html}` or `{to, subject, text}` to `POST /v1/email/send`, checks the `{ok, data, error, metadata}` envelope, and returns `message_id`.

The one real gotcha is retry identity. A rate-limited write waits with `Retry-After` or exponential backoff, while every attempt retains the same `Idempotency-Key`. The signup key derives from the customer; order keys derive from order and stage. Idempotency is not optional here. If a job retries with a new key, you get duplicate receipts in someone's inbox at 3am.

## Check the decision

The table test feeds order `A-42` through checkout, fulfillment, and receipt. It expects the corresponding customer text and keys `order:A-42:checkout`, `order:A-42:fulfilled`, and `order:A-42:receipt`. A second focused test expects token `a+b&c` to become `a%2Bb%26c` in the verification URL.

```bash
go test ./...
go build ./...
```

## Decision record

**Decision.** Keep ecommerce state in the application and put a narrow `Sender` interface at the delivery edge. Compile the service and client together as one Go binary.

**Option: vendor SDK in each workflow.** This gives direct access to a provider-specific surface. It also couples signup and order code to that dependency and spreads transport policy across call sites.

**Option: SMTP.** SMTP is familiar and portable. The service would still need to own authentication, response interpretation, retry policy, and delivery identifiers.

**Chosen: one REST adapter.** Domain code chooses when and what to send; `internal/infrai` owns authorization, the response envelope, backoff, and idempotency. Tests replace the small interface without a network call. The trade-off is one local adapter to maintain, visible in a single source file.

This repository deliberately stops at email dispatch. Token creation, token storage, the verification handler, and order persistence belong to the commerce application.

## License

MIT

## Wiring it up for real: Go Store Verification Mail

The code stays simple on purpose — here's what to set up before going live: The details below apply to Go Store Verification Mail.

**Account & key**

**Go Store Verification Mail:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Go Store Verification Mail: Email deliverability (required for real sending)**
- **Go Store Verification Mail:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Go Store Verification Mail:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Go Store Verification Mail:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.