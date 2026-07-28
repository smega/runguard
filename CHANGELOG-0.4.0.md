# RunGuard 0.4.0

## Added
- Native Stripe Checkout Session creation for paid plans.
- Native Stripe Customer Portal session creation.
- Stripe webhook signature verification using `Stripe-Signature` and the configured webhook secret.
- Subscription metadata mapping (`user_id`, `email`, `plan_id`) for long-term Stripe lifecycle sync.
- Billing state persistence fields for checkout attempts and last Stripe event details.
- `stripe_price_id`, `stripe_secret_key`, success/cancel URLs, and portal return URL in config.

## Changed
- Billing flow now uses the Stripe API instead of static checkout and portal links.
- Billing webhook endpoint now expects real Stripe events rather than custom JSON payloads.
- Sample config was sanitized and updated to the long-term Stripe model.

## Notes
- You will need to run `go mod download` / `go build` in an environment with network access to fetch the Stripe Go SDK.
- The frontend billing UI continues to call the existing backend endpoints, so no UI flow changes are required for checkout and portal launch.
