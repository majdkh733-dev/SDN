package migrate

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConcurrentApplyDoesNotCorruptLedger(t *testing.T) {
	url := os.Getenv("MIGRATE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("MIGRATE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- Apply(ctx, pool)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var versions int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions < 8 {
		t.Fatalf("expected the embedded migrations, got %d", versions)
	}
	if err := Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
}
