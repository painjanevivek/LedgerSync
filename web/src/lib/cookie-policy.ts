/** One runtime policy for cookie prefixes and transport attributes. */
export function authenticationCookiePolicy(environment: Readonly<Record<string, string | undefined>> = process.env) {
  const deployment = (environment.LEDGERSYNC_DEPLOYMENT_ENV ?? environment.LEDGERSYNC_ENV ?? environment.NODE_ENV ?? "development").trim().toLowerCase();
  // A production Next build can run on the explicitly local HTTP workspace.
  const secure = !(deployment === "development" && environment.LEDGERSYNC_COOKIE_SECURE === "false");
  const production = deployment === "production" || deployment === "prod";
  return {
    secure,
    sessionName: production ? "__Host-ledgersync_session" : "ledgersync_session",
    transactionName: production ? "__Secure-ledgersync_oidc_transaction" : "ledgersync_oidc_transaction",
  };
}
