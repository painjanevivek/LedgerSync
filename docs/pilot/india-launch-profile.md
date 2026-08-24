# India controlled-pilot launch profile

**Decision date:** 2026-08-24

**Status:** approved product and architecture baseline; managed-environment evidence remains required

**Primary region:** AWS Asia Pacific (Mumbai), `ap-south-1`

**Secondary backup region:** AWS Asia Pacific (Hyderabad), `ap-south-2`

**Pilot currency:** INR, stored as integer paise

This profile converts the pilot questions into an enforceable release boundary. It is not evidence that an AWS environment, Cognito pool, legal review, restore drill, or customer pilot already exists. Those items remain explicit deployment and approval gates.

## Buyer, users, and surfaces

- The initial buyer is the CTO or Head of Engineering at an India-based vertical-SaaS or fintech-infrastructure company that needs closed-loop balances, credits, internal payouts, or wallet-like ledger accounts.
- Integration engineers are the primary daily API users. Finance, operations, support, and engineering use the operator console for investigation, reconciliation, audit, and incident response.
- The API is the core product and must be validated first.
- The invite-only operator console is a required launch companion. It is not a public signup, consumer wallet, or self-service tenant-administration product.
- Two or three design partners may participate, onboarded sequentially. The first partner must complete its limited observation window before another partner is enabled.

## Financial boundary

Only internal, same-tenant, same-currency transfers between LedgerSync ledger accounts are in scope. LedgerSync does not provide bank rails, card processing, cash-out, foreign exchange, cross-border settlement, custody, or a stored-value product. External funding and settlement remain the design partner's responsibility.

Money remains an integer at rest and in Go. JSON carries canonical decimal strings so JavaScript cannot round a signed 64-bit value. For INR, one major unit is 100 paise.

| Control | Approved value | Stored value |
|---|---:|---:|
| Minimum transfer | ₹1 | `100` paise |
| Maximum single transfer | ₹100,000 | `10000000` paise |
| Actor rolling 24 hours | ₹500,000 | `50000000` paise |
| Source account rolling 24 hours | ₹500,000 | `50000000` paise |
| Tenant rolling 24 hours | ₹5,000,000 | `500000000` paise |
| Overdraft | prohibited | enforced inside the posting transaction |
| Cross-tenant transfer | prohibited | tenant predicates and authorization deny it |

Limits may vary only through the reviewed, audited provisioning workflow. An ordinary product API cannot edit pilot policy. The rolling window is the immediately preceding 24 hours in UTC, evaluated transactionally against posted transfers.

## Cognito identity profile

Amazon Cognito user pools in Mumbai are the production identity provider. Cognito documents separate authorization-code and client-credentials app-client modes; one app client cannot combine the client-credentials grant with authorization-code or implicit grants. The operator and machine identities therefore use different app clients.

### Operator console

- Authorization-code flow with S256 PKCE.
- Mandatory MFA enforced in the Cognito user-pool/app-client policy.
- Self-registration disabled; every operator is invited through an approved change.
- The BFF validates issuer, signature, ID-token audience, expiry, nonce, and `token_use=id`.
- `LEDGERSYNC_OIDC_SUBJECT_PERMISSIONS` maps the Cognito subject to one tenant and allowlisted roles/scopes on the server. Token claims or browser fields cannot select the tenant.
- The BFF creates an HttpOnly, Secure, SameSite session and sends a separate short-lived signed actor assertion to the private API.

### Partner API and BFF workload

- Client-credentials flow with confidential app clients and narrow custom resource scopes.
- The Go API validates issuer, signature, expiry, `token_use=access`, `client_id`, resource `aud`, and allowlisted scopes.
- `LEDGERSYNC_OIDC_CLIENT_TENANT_MAP` maps each approved app-client ID to one tenant on the server.
- The BFF workload client has only `bff:act-as-user`. That scope is unusable without a valid, unreplayed actor assertion.
- Client secrets and renewed workload tokens are delivered through AWS Secrets Manager or an equivalent managed workload mechanism; production refuses a static environment token.

AWS notes that Cognito access tokens use `client_id` for the app client and include `aud` only when resource binding is requested. LedgerSync intentionally requires both so a valid token for another resource is rejected.

## AWS network and service boundary

The Phase 2 infrastructure implementation must encode this topology:

- Internet clients reach CloudFront or an application load balancer through AWS WAF.
- The console and partner API expose HTTPS only. Partner IP allowlists are applied when a design partner has stable egress ranges.
- Next.js/BFF workloads run in private application subnets. The Go API, worker, reconciliation, provisioning, and administrative workloads remain private.
- Amazon RDS for PostgreSQL uses Multi-AZ deployment and is the only financial authority.
- Amazon ElastiCache for Redis is private, disposable, encrypted, and rebuildable from PostgreSQL.
- Security groups permit only necessary workload-to-workload paths. PostgreSQL, Redis, diagnostics, provisioning, and operational administration have no public listener.
- AWS KMS protects managed storage and secret keys. Secrets Manager stores database/app-client credentials with rotation and audit evidence.
- Logs, traces, alarms, and security findings stay in approved India regions unless legal/security explicitly approve another destination.

AWS lists Mumbai as a three-Availability-Zone region and provides Cognito, RDS, and other regional endpoints there. Hyderabad is an India region suitable for encrypted backup copies, but the pilot does not introduce active-active financial writes.

## Recovery and continuity objectives

| Failure class | RPO | RTO | Required proof |
|---|---:|---:|---|
| Availability-zone failure | 0 for committed PostgreSQL transactions | ≤15 minutes | Multi-AZ failover exercise and reconciled writes |
| Database corruption or operator error | ≤5 minutes | ≤60 minutes | Provider-backed isolated PITR restore |

Before production traffic, perform at least one provider-backed isolated restore. After restoration, rebuild Redis from PostgreSQL, run full reconciliation, verify zero unexplained mismatches, and only then reopen writes. Keep encrypted backup copies in Hyderabad without adding a second financial writer.

## Security, privacy, and regulatory posture

- Build for the Digital Personal Data Protection Act and the notified Digital Personal Data Protection Rules, 2025: maintain a data inventory, lawful-purpose record, privacy notice, processor agreements, retention/deletion controls, breach workflow, and data-principal request process.
- Maintain synchronized clocks, incident records, security logs, and a CERT-In reporting process. The incident commander must be capable of assessing and reporting an applicable incident within the prescribed window.
- Encrypt traffic and storage, require MFA for privileged users, maintain least-privilege workload/database roles, review access, manage vulnerabilities, preserve immutable financial evidence, and test incident response.
- Target SOC 2 Type I readiness during the pilot. Start an audit only when the control environment and enterprise sales path justify it.
- Cardholder data is prohibited, so PCI DSS is outside the declared product scope.
- LedgerSync is positioned contractually as non-custodial ledger software. It is not represented as a payment-system operator or holder of funds.
- Indian counsel must confirm the final DPDP, CERT-In, outsourcing, sectoral, data-residency, and RBI perimeter before production. Regulated partners may contractually flow down requirements even when LedgerSync is not directly regulated as a payment-system provider.

## Capacity and service objectives

- Support at least 10,000 accounts per tenant and approximately 30,000 across the pilot.
- Demonstrate 10 TPS for 60 minutes and a 50 TPS burst for 10 minutes.
- Stress to 100 TPS to measure headroom and graceful degradation; 100 TPS is not a launch SLO.
- Target 50,000–250,000 genuine pilot transfers over approximately 30 days.
- Transfer commit latency: p95 under 500 ms and p99 under 1 second with healthy dependencies.
- Balance-read latency: p95 under 200 ms.
- Monthly API availability: at least 99.9%.
- Immediate post-transfer reads return at least the committed balance version or a truthful unavailable response.

## Pilot graduation evidence

The pilot does not graduate until all of the following are evidenced:

- zero duplicate financial movement, unbalanced journals, unexplained reconciliation mismatches, cross-tenant access, or stale-as-current balance responses;
- every posted transfer has immutable postings, audit evidence, and an idempotent outcome;
- provider-backed restore meets RPO/RTO and reconciles with zero differences;
- no unresolved critical or high security findings;
- 100% MFA coverage for privileged users;
- median sandbox time to first successful transfer below one business day;
- median operator time to locate complete transfer evidence below five minutes;
- at least 30 consecutive days of controlled traffic without an unrecoverable financial incident.

## Primary references

- [AWS Cognito app-client grants and PKCE](https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-settings-client-apps.html)
- [AWS Cognito access-token claims](https://docs.aws.amazon.com/cognito/latest/developerguide/amazon-cognito-user-pools-using-the-access-token.html)
- [AWS Cognito JWT verification](https://docs.aws.amazon.com/cognito/latest/developerguide/amazon-cognito-user-pools-using-tokens-verifying-a-jwt.html)
- [AWS regions and Availability Zones](https://docs.aws.amazon.com/global-infrastructure/latest/regions/aws-regions.html)
- [MeitY Digital Personal Data Protection Rules, 2025](https://www.meity.gov.in/documents/act-and-policies/digital-personal-data-protection-rules-2025-gDOxUjMtQWa)
- [CERT-In directions under section 70B](https://www.cert-in.org.in/Directions70B.jsp)
- [RBI storage of payment-system data direction](https://rbi.org.in/Scripts/NotificationUser.aspx?Id=11244)
