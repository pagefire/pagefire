package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pagefire/pagefire/internal/store"
	"github.com/pagefire/pagefire/internal/store/migrations"
	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"
)

// newStressStore creates a fresh SQLite store in a temp directory with
// WAL mode, busy_timeout applied via DSN (so every pooled connection gets it),
// and migrations applied. The caller should defer the returned cleanup func.
//
// NOTE: The production New() function sets busy_timeout via PRAGMA after open,
// which only affects one connection in the pool. For stress testing we use the
// DSN _pragma approach so the timeout is set on every connection the pool opens.
// This is also a signal that the production code should be fixed similarly.
func newStressStore(t *testing.T) (*SQLiteStore, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "pagefire-stress-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}

	dbPath := filepath.Join(dir, "stress.db")

	// Open with DSN-level pragmas so every connection in the pool inherits them.
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("opening sqlite: %v", err)
	}

	s := &SQLiteStore{db: db}

	ctx := context.Background()
	goose.SetBaseFS(migrations.SQLiteMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		s.Close()
		os.RemoveAll(dir)
		t.Fatalf("setting goose dialect: %v", err)
	}
	if err := goose.UpContext(ctx, db, "sqlite"); err != nil {
		s.Close()
		os.RemoveAll(dir)
		t.Fatalf("running migrations: %v", err)
	}

	cleanup := func() {
		s.Close()
		os.RemoveAll(dir)
	}
	return s, cleanup
}

// createStressService creates a service prerequisite for alert creation.
func createStressService(t *testing.T, s *SQLiteStore) string {
	t.Helper()
	ctx := context.Background()

	// escalation_policy_id is NOT NULL but has no FK in SQLite, so a placeholder works.
	svc := &store.Service{
		Name:               "stress-test-service",
		EscalationPolicyID: "ep-placeholder",
	}
	if err := s.Services().Create(ctx, svc); err != nil {
		t.Fatalf("creating service: %v", err)
	}
	return svc.ID
}

// --------------------------------------------------------------------------
// Test 1: Concurrent alert creation
// --------------------------------------------------------------------------

func TestStress_ConcurrentAlertCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	s, cleanup := newStressStore(t)
	defer cleanup()

	serviceID := createStressService(t, s)
	ctx := context.Background()

	const goroutines = 10
	const alertsPerGoroutine = 100
	const totalAlerts = goroutines * alertsPerGoroutine

	var (
		wg       sync.WaitGroup
		errCount atomic.Int64
	)

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer wg.Done()
			for i := 0; i < alertsPerGoroutine; i++ {
				a := &store.Alert{
					ServiceID: serviceID,
					Summary:   fmt.Sprintf("stress alert g%d-i%d", gID, i),
					Source:    "stress-test",
				}
				if err := s.Alerts().Create(ctx, a); err != nil {
					errCount.Add(1)
					t.Errorf("goroutine %d, alert %d: Create failed: %v", gID, i, err)
				}
			}
		}(g)
	}

	wg.Wait()

	if errCount.Load() > 0 {
		t.Fatalf("%d alert creation errors occurred under concurrency", errCount.Load())
	}

	// Verify all alerts exist.
	alerts, err := s.Alerts().List(ctx, store.AlertFilter{ServiceID: serviceID})
	if err != nil {
		t.Fatalf("listing alerts: %v", err)
	}
	if len(alerts) != totalAlerts {
		t.Fatalf("expected %d alerts, got %d", totalAlerts, len(alerts))
	}

	t.Logf("successfully created %d alerts from %d concurrent goroutines", totalAlerts, goroutines)
}

// --------------------------------------------------------------------------
// Test 2: Concurrent acknowledge and resolve
// --------------------------------------------------------------------------

func TestStress_ConcurrentAckResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	s, cleanup := newStressStore(t)
	defer cleanup()

	serviceID := createStressService(t, s)
	ctx := context.Background()

	const numAlerts = 100

	// Pre-create alerts.
	alertIDs := make([]string, numAlerts)
	for i := 0; i < numAlerts; i++ {
		a := &store.Alert{
			ServiceID: serviceID,
			Summary:   fmt.Sprintf("ack-resolve alert %d", i),
			Source:    "stress-test",
		}
		if err := s.Alerts().Create(ctx, a); err != nil {
			t.Fatalf("creating alert %d: %v", i, err)
		}
		alertIDs[i] = a.ID
	}

	// Create a user for the acknowledged_by FK constraint.
	ackUser := &store.User{
		Name:     "Stress Acker",
		Email:    "stress-ack@test.local",
		Role:     "user",
		Timezone: "UTC",
		IsActive: true,
	}
	if err := s.Users().Create(ctx, ackUser); err != nil {
		t.Fatalf("creating ack user: %v", err)
	}

	// Concurrently acknowledge and resolve all alerts.
	// Use 4 goroutines for ack, 4 for resolve — they race against each other.
	var wg sync.WaitGroup

	// Acknowledge goroutines — each handles a slice of alerts.
	const ackWorkers = 4
	chunkSize := numAlerts / ackWorkers
	for w := 0; w < ackWorkers; w++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			end := start + chunkSize
			if end > numAlerts {
				end = numAlerts
			}
			for i := start; i < end; i++ {
				if err := s.Alerts().Acknowledge(ctx, alertIDs[i], ackUser.ID); err != nil {
					t.Errorf("ack alert %d: %v", i, err)
				}
			}
		}(w * chunkSize)
	}

	// Resolve goroutines — same alerts, racing with ack.
	const resolveWorkers = 4
	for w := 0; w < resolveWorkers; w++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			end := start + chunkSize
			if end > numAlerts {
				end = numAlerts
			}
			for i := start; i < end; i++ {
				if err := s.Alerts().Resolve(ctx, alertIDs[i]); err != nil {
					t.Errorf("resolve alert %d: %v", i, err)
				}
			}
		}(w * chunkSize)
	}

	wg.Wait()

	// Verify: every alert must be in acknowledged or resolved state — never triggered.
	var acked, resolved int
	for _, id := range alertIDs {
		a, err := s.Alerts().Get(ctx, id)
		if err != nil {
			t.Fatalf("getting alert %s: %v", id, err)
		}
		switch a.Status {
		case store.AlertStatusAcknowledged:
			acked++
		case store.AlertStatusResolved:
			resolved++
		default:
			t.Errorf("alert %s has unexpected status %q", id, a.Status)
		}
	}

	t.Logf("final states: %d acknowledged, %d resolved (of %d total)", acked, resolved, numAlerts)

	if acked+resolved != numAlerts {
		t.Fatalf("expected %d alerts in terminal states, got %d", numAlerts, acked+resolved)
	}
}

// --------------------------------------------------------------------------
// Test 3: List queries under concurrent write load
// --------------------------------------------------------------------------

func TestStress_ListUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	s, cleanup := newStressStore(t)
	defer cleanup()

	serviceID := createStressService(t, s)
	ctx := context.Background()

	const totalAlerts = 1000
	const writerGoroutines = 5
	const alertsPerWriter = totalAlerts / writerGoroutines
	const readerGoroutines = 5

	// done signals writers are finished, so readers know when to stop.
	done := make(chan struct{})

	var writerWg sync.WaitGroup

	// Writers: create alerts concurrently.
	writerWg.Add(writerGoroutines)
	for g := 0; g < writerGoroutines; g++ {
		go func(gID int) {
			defer writerWg.Done()
			for i := 0; i < alertsPerWriter; i++ {
				a := &store.Alert{
					ServiceID: serviceID,
					Summary:   fmt.Sprintf("list-stress g%d-i%d", gID, i),
					Source:    fmt.Sprintf("source-%d", gID%3), // 3 distinct sources
				}
				if err := s.Alerts().Create(ctx, a); err != nil {
					t.Errorf("writer %d alert %d: %v", gID, i, err)
				}
			}
		}(g)
	}

	// Readers: run list queries with various filters while writes are happening.
	var (
		readerWg       sync.WaitGroup
		listErrors     atomic.Int64
		totalListCalls atomic.Int64
	)

	readerWg.Add(readerGoroutines)
	for r := 0; r < readerGoroutines; r++ {
		go func(rID int) {
			defer readerWg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}

				// Vary filters across readers.
				var filter store.AlertFilter
				switch rID % 5 {
				case 0:
					// No filter — list all.
					filter = store.AlertFilter{ServiceID: serviceID}
				case 1:
					// Filter by status.
					filter = store.AlertFilter{ServiceID: serviceID, Status: store.AlertStatusTriggered}
				case 2:
					// Filter by source.
					filter = store.AlertFilter{ServiceID: serviceID, Source: "source-1"}
				case 3:
					// Paginated query.
					filter = store.AlertFilter{ServiceID: serviceID, Limit: 50, Offset: 10}
				case 4:
					// Search filter.
					filter = store.AlertFilter{ServiceID: serviceID, Search: "list-stress"}
				}

				alerts, err := s.Alerts().List(ctx, filter)
				totalListCalls.Add(1)
				if err != nil {
					listErrors.Add(1)
					t.Errorf("reader %d: list error: %v", rID, err)
					continue
				}

				// Consistency check: no alert should have an empty ID.
				for _, a := range alerts {
					if a.ID == "" {
						t.Errorf("reader %d: got alert with empty ID", rID)
					}
					if a.ServiceID != serviceID {
						t.Errorf("reader %d: got alert with wrong service_id %q", rID, a.ServiceID)
					}
				}
			}
		}(r)
	}

	// Wait for writers to finish, then signal readers to stop.
	writerWg.Wait()
	close(done)
	readerWg.Wait()

	if listErrors.Load() > 0 {
		t.Fatalf("%d list errors during concurrent reads/writes", listErrors.Load())
	}

	// Final consistency: all 1000 alerts should exist.
	allAlerts, err := s.Alerts().List(ctx, store.AlertFilter{ServiceID: serviceID})
	if err != nil {
		t.Fatalf("final list: %v", err)
	}
	if len(allAlerts) != totalAlerts {
		t.Fatalf("expected %d alerts after writes, got %d", totalAlerts, len(allAlerts))
	}

	t.Logf("completed %d list queries during concurrent writes of %d alerts with 0 errors",
		totalListCalls.Load(), totalAlerts)
}
