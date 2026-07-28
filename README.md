# make user admin

// in mongosh, adjust the email to yours:
db.users.updateOne(
  { email: "you@example.com" },
  { $set: { is_admin: true } }
)


## Stripe billing

RunGuard now supports long-term Stripe integration using:
- Checkout Sessions for subscription signup
- Billing Portal sessions for customer self-service
- verified Stripe webhooks for subscription lifecycle updates

Configure these fields in `backend/config.yml`:
- `billing.stripe_secret_key`
- `billing.webhook_secret`
- `billing.success_url`
- `billing.cancel_url`
- `billing.portal_return_url`
- `billing.plans[].stripe_price_id`

Recommended Stripe webhook events:
- `checkout.session.completed`
- `customer.subscription.created`
- `customer.subscription.updated`
- `customer.subscription.deleted`
- `invoice.payment_failed`
- `invoice.paid`
