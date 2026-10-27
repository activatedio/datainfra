package gorm

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// A Config logged, by Object or after Redacted, never carries its password:
// setup and teardown log the app's Config at info, and a migrate Job's log is
// read far more widely than its secret.
func TestConfigLogsWithoutItsPassword(t *testing.T) {
	c := Config{Dialect: "postgres", Host: "pg", Port: 5432, Username: "app", Password: "s3cret-pw", Name: "app", Schema: "analytics"}
	var buf bytes.Buffer
	log := zerolog.New(&buf)
	log.Info().Object("appConfig", c).Msg("setup")
	log.Info().Object("appConfig", &c).Msg("teardown")
	out := buf.String()
	if strings.Contains(out, "s3cret-pw") {
		t.Fatalf("the password was logged: %s", out)
	}
	if !strings.Contains(out, `"password":"[redacted]"`) || !strings.Contains(out, `"host":"pg"`) || !strings.Contains(out, `"schema":"analytics"`) {
		t.Fatalf("the config is not logged as expected: %s", out)
	}
	if r := c.Redacted(); r.Password != "[redacted]" || c.Password != "s3cret-pw" || r.Host != "pg" {
		t.Fatalf("Redacted = %+v, original %+v", r, c)
	}
	if (Config{}).Redacted().Password != "" {
		t.Fatal("an empty password became a placeholder")
	}
	buf.Reset()
	log.Info().Object("appConfig", Config{Host: "pg"}).Msg("setup")
	if strings.Contains(buf.String(), "password") {
		t.Fatalf("an empty password was logged as one: %s", buf.String())
	}
	if strings.Contains(buf.String(), "Conns") || strings.Contains(buf.String(), "connMax") {
		t.Fatalf("an unset pool field was logged: %s", buf.String())
	}
	buf.Reset()
	idle, open, idleTime, lifetime := 10, 25, 5*time.Minute, 30*time.Minute
	log.Info().Object("appConfig", Config{MaxIdleConns: &idle, MaxOpenConns: &open, ConnMaxIdleTime: &idleTime, ConnMaxLifetime: &lifetime}).Msg("setup")
	for _, want := range []string{`"maxIdleConns":10`, `"maxOpenConns":25`, `"connMaxIdleTime":300000`, `"connMaxLifetime":1800000`} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("%s is not logged: %s", want, buf.String())
		}
	}
}
