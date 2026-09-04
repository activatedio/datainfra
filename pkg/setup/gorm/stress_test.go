package gorm_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	datagorm "github.com/activatedio/datainfra/pkg/data/gorm"
	"github.com/activatedio/datainfra/pkg/setup"
	"github.com/activatedio/datainfra/pkg/setup/gorm"
)

// TestStress_ConcurrentDatabaseDDL reproduces the failure seen running
// `go test ./...` against YugabyteDB at default parallelism: many processes
// each creating and dropping their own test database at once.
//
// CREATE/DROP DATABASE are not transactional on YugabyteDB, so a read
// restart can be reported after the keyspace has already been created,
// leaving an orphan that makes the retry fail with "Keyspace 'x' already
// exists" (XX000) while pg_database has no row for it.
func TestStress_ConcurrentDatabaseDDL(t *testing.T) {

	// YugabyteDB-specific, and not part of the default unit run: it needs a
	// YSQL endpoint on 5433 and takes tens of seconds. Skipped unless one is
	// reachable, so `go test ./...` on a machine with only postgres still
	// passes.
	if c, err := net.DialTimeout("tcp", "127.0.0.1:5433", 2*time.Second); err != nil {
		t.Skip("no YugabyteDB on 127.0.0.1:5433; skipping concurrent-DDL regression")
	} else {
		_ = c.Close()
	}

	workers := 12
	rounds := 3
	if v := os.Getenv("STRESS_WORKERS"); v != "" {
		fmt.Sscanf(v, "%d", &workers)
	}
	if v := os.Getenv("STRESS_ROUNDS"); v != "" {
		fmt.Sscanf(v, "%d", &rounds)
	}

	ctx := context.Background()
	var setupErrs, teardownErrs, ok int64
	var mu sync.Mutex
	var msgs []string

	record := func(phase string, err error) {
		mu.Lock()
		defer mu.Unlock()
		if len(msgs) < 12 {
			msgs = append(msgs, phase+": "+err.Error())
		}
	}

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				name := fmt.Sprintf("stress_%d_%d_%d_%d", time.Now().UnixMilli(), os.Getpid(), w, r)
				params := gorm.SetupParams{
					OwnerConfig: &gorm.OwnerGormConfig{Config: datagorm.Config{
						Dialect: "postgres", Host: "127.0.0.1", Port: 5433,
						Username: "yugabyte", Password: "yugabyte", Name: "yugabyte",
					}},
					AppConfig: &datagorm.Config{
						Dialect: "postgres", Host: "127.0.0.1", Port: 5433,
						Username: "yugabyte", Password: "yugabyte", Name: name,
					},
				}
				s := gorm.NewSetup(params)
				if err := s.Setup(ctx, setup.Params{}); err != nil {
					atomic.AddInt64(&setupErrs, 1)
					record("setup", err)
					continue
				}
				if err := s.Teardown(ctx); err != nil {
					atomic.AddInt64(&teardownErrs, 1)
					record("teardown", err)
					continue
				}
				atomic.AddInt64(&ok, 1)
			}
		}(w)
	}
	wg.Wait()

	t.Logf("STRESSRESULT workers=%d rounds=%d ok=%d setup_errs=%d teardown_errs=%d",
		workers, rounds, ok, setupErrs, teardownErrs)
	for _, m := range msgs {
		t.Logf("STRESSERR %s", m)
	}
	if setupErrs+teardownErrs > 0 {
		t.Errorf("STRESSFAIL %d setup + %d teardown errors", setupErrs, teardownErrs)
	}
}
