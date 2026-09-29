package gorm

import (
	"strings"
	"testing"
)

func TestPostgresDSNCarriesTheSchema(t *testing.T) {
	cfg := &Config{Host: "h", Port: 5432, Username: "u", Password: "p", Name: "d", Schema: "fleet"}
	if dsn := postgresDSN(cfg); !strings.HasSuffix(dsn, " search_path=fleet") {
		t.Errorf("dsn = %q", dsn)
	}
	cfg.Schema = ""
	if dsn := postgresDSN(cfg); strings.Contains(dsn, "search_path") {
		t.Errorf("no schema, dsn = %q", dsn)
	}
}

func TestValidateSchema(t *testing.T) {
	for _, ok := range []string{"", "fleet", "review_items", "_x9"} {
		if err := ValidateSchema(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"Fleet", "9lives", "a b", "fleet;drop", "a-b", strings.Repeat("a", 64)} {
		if err := ValidateSchema(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if _, err := NewDB(&Config{Dialect: DialectPostgres, Schema: "bad schema"}); err == nil {
		t.Error("NewDB opened a connection for a bad schema")
	}
}
