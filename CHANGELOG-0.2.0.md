# RunGuard 0.2.0

This archive adds a productization pass focused on pricing/billing flow scaffolding and project/incident workflow.

## Added
- Projects backend model and API
- Incidents backend model and API with acknowledge/resolve states
- Pricing and billing endpoints driven by config
- Pricing page, Projects page, Incidents page
- Account page billing actions
- Jobs now support project assignment in backend and basic frontend create/edit support
- Default project creation for new users

## Notes
- Billing is checkout/portal URL driven from config.yml so you can wire Stripe links without changing code.
- This does not yet include Stripe webhook processing or automatic plan switching after payment.
- Recovery alerts are not yet sent as separate email/webhook notifications; incidents are resolved on successful heartbeat.
