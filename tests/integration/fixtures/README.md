# Integration fixtures

Fixtures are created only inside the disposable database named by
`LEDGERSYNC_TEST_DATABASE_URL`. Tests must never default to a shared, local, or
production database. Redis test state must use a unique prefix per test run.
