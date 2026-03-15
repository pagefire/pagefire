package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pagefire/pagefire/internal/store"
	"github.com/pagefire/pagefire/internal/store/migrations"
	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements store.Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// New opens a SQLite database and configures WAL mode and foreign keys.
// Pragmas are set via DSN parameters so they apply to every connection
// in the pool, not just the first one (fixes SQLITE_BUSY under concurrency).
func New(dbPath string) (*SQLiteStore, error) {
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLiteStore) Migrate(ctx context.Context) error {
	goose.SetBaseFS(migrations.SQLiteMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.UpContext(ctx, s.db, "sqlite")
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Users() store.UserStore               { return &userStore{db: s.db} }
func (s *SQLiteStore) Teams() store.TeamStore               { return &teamStore{db: s.db} }
func (s *SQLiteStore) Services() store.ServiceStore          { return &serviceStore{db: s.db} }
func (s *SQLiteStore) EscalationPolicies() store.EscalationPolicyStore {
	return &escalationPolicyStore{db: s.db}
}
func (s *SQLiteStore) Schedules() store.ScheduleStore        { return &scheduleStore{db: s.db} }
func (s *SQLiteStore) Alerts() store.AlertStore              { return &alertStore{db: s.db} }
func (s *SQLiteStore) Notifications() store.NotificationStore { return &notificationStore{db: s.db} }
func (s *SQLiteStore) Incidents() store.IncidentStore        { return &incidentStore{db: s.db} }

// DB returns the underlying *sql.DB, needed by the session store.
func (s *SQLiteStore) DB() *sql.DB { return s.db }
