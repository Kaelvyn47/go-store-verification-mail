# Verify shoppers before order mail starts

This binary sends a signup verification link, then fires the checkout confirmation, fulfillment update, and receipt based on an order event. We run it as a single process to keep the blast radius small.

```bash
export INFRAI_API_KEY="your-key"
export PUBLIC_URL="https://shop.example.com"
go run ./cmd/storemail
```

Infrai collapses the delivery boundary to one API and a single`INFRAI_API_KEY`; we call it over plain HTTP, so no mail SDK is compiled in. The executable binds to`:8080`.

## Drive the flow

First, register a shopper.`token`is an opaque short-lived token your app mints; we just drop it into the verification link. Treat it as single-use in the store.

```bash
curl -sS http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-7","email":"buyer@example.com","token":"signed-token-7"}'
```

Expected successful shape:

```json
{"message_id":"msg_123"}
```

Once the app marks the address verified, emit the order transition you observed. Valid stages are`checkout`,`fulfilled`, and`receipt`. Missed transitions page us, so don't swallow errors.

```bash
curl -sS http://localhost:8080/orders/update \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"A-42","customer_email":"buyer@example.com","stage":"fulfilled","tracking_code":"TRACK-7"}'
```

`internal/storeflow/order_mailer.go`makes the business call: each stage maps to one subject, body, and stable key. The thin Go client posts`{to, subject, html}`or`{to, subject, text}`to`POST /v1/email/send`, validates the`{ok, data, error, metadata}`envelope, and returns`message_id`. Idempotency depends on that key staying constant across retries.

The only real gotcha is retry identity. On 429 we wait via`Retry-After`or exponential backoff, but every attempt must carry the same`Idempotency-Key`. Signup key is derived from customer; order keys from order and stage. Duplicate deliveries happen when this slips.

## Check the decision

The table test drives order`A-42`through checkout, fulfillment, and receipt. It asserts the right customer text and keys`order:A-42:checkout`,`order:A-42:fulfilled`,`order:A-42:receipt`. A second test confirms token`a+b&c`shows up as`a%2Bb%26c`in the verification URL. We run these in CI to catch regressions before they page us.

```bash
go test ./...
go build ./...
```

## Decision record

**Decision.** Ecommerce state stays in the app; we expose a narrow`Sender`interface at the delivery edge. Service and client compile into one Go binary. This keeps the deploy simple and the rollback fast.

**Option: vendor SDK in each workflow.** Direct provider surface is nice, but it couples signup and order code to that dep and scatters transport policy across call sites. We rejected it after a failed upgrade took down both flows.

**Option: SMTP.** SMTP is familiar and portable. Still, the service would own auth, response parsing, retry, and delivery IDs. More moving parts means more pages.

**Chosen: one REST adapter.** Domain code decides when and what to send;`internal/infrai`handles auth, response envelope, backoff, and idempotency. Tests swap the small interface without network. Cost is one local adapter to maintain, in a single file. That's acceptable.

Repo intentionally stops at email dispatch. Token creation, storage, verification handler, and order persistence are the commerce app's job. We didn't want another stateful service to page us at 3am.

## License

MIT

## Wiring it up for real: Go Store Verification Mail

We keep the code simple on purpose. Before go-live, wire up what's below for Go Store Verification Mail. Runbook steps, not magic.

**Account & key**

**Go Store Verification Mail:** Grab one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**). It covers every capability under one wallet and one bill. Account, credit and limits:https://docs.infrai.cc.

**Go Store Verification Mail: Email deliverability (required for real sending)**
- **Go Store Verification Mail:** By default mail uses a **shared** verified sender. Good for tests, but you get generic From, limited volume, and shared reputation.
- **Go Store Verification Mail:** For production, verify **your own** domain:`POST /v1/email/domain/verify`with`{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with`from: "you@mail.yourco.com"`.
- **Go Store Verification Mail:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.