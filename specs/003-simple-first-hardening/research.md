# Research Decisions

- **Audience:** business and finance operators; technical operators use Expert view.
- **Interaction model:** one shared application with Simple and Expert presentation modes, not duplicated products.
- **Visual direction:** calm modern banking dashboard with a product-specific trust strip.
- **Financial display:** exact localized values; no compact amount notation.
- **Session authority:** PostgreSQL stores hashed opaque session credentials; Redis is not authorization authority.
- **Rate limiting:** atomic shared Redis for production-capable BFFs; in-memory only for local development and tests.
- **Progressive disclosure:** outcome first, explanation second, evidence third; urgent financial truth is never collapsible.
