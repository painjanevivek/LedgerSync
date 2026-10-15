package config

import "testing"

func TestReplicaIdentityAndPoolBudget(t *testing.T) {
	t.Setenv("LEDGERSYNC_NODE_ID", "api-02")
	t.Setenv("LEDGERSYNC_DATABASE_MAX_OPEN_CONNS", "2")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NodeIdentity("api") != "ledgersync/api/api-02" || cfg.DatabaseMaxOpenConns != 2 {
		t.Fatal("replica configuration was not retained")
	}
	t.Setenv("LEDGERSYNC_NODE_ID", "same node with spaces")
	if _, err := Load(); err == nil {
		t.Fatal("invalid identity was accepted")
	}
	t.Setenv("LEDGERSYNC_NODE_ID", "api-02")
	t.Setenv("LEDGERSYNC_DATABASE_MAX_OPEN_CONNS", "65")
	if _, err := Load(); err == nil {
		t.Fatal("unbounded pool was accepted")
	}
}
