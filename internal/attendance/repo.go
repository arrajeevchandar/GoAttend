package attendance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Repository persists attendance data in Postgres.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a repo.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// UpsertDevice ensures a device record exists.
func (r *Repository) UpsertDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return errors.New("device id required")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO devices (device_id)
		VALUES ($1)
		ON CONFLICT (device_id) DO NOTHING
	`, deviceID)
	return err
}

// SaveRefreshToken stores a refresh token for rotation checks.
func (r *Repository) SaveRefreshToken(ctx context.Context, deviceID, token string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (device_id, token, expires_at)
		VALUES ($1, $2, $3)
	`, deviceID, token, expiresAt)
	return err
}

// RevokeRefreshToken marks a token revoked.
func (r *Repository) RevokeRefreshToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked = TRUE WHERE token = $1`, token)
	return err
}

// RecentEvent returns a recent event within the provided window.
func (r *Repository) RecentEvent(ctx context.Context, userID, deviceID string, window time.Duration) (*Event, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, device_id, occurred_at, location, image_url, status, match_score, created_at
		FROM attendance_events
		WHERE user_id = $1 AND device_id = $2 AND occurred_at >= NOW() - ($3 * interval '1 second')
		ORDER BY occurred_at DESC
		LIMIT 1
	`, userID, deviceID, window.Seconds())
	var evt Event
	if err := row.Scan(&evt.ID, &evt.UserID, &evt.DeviceID, &evt.When, &evt.Location, &evt.ImageURL, &evt.Status, &evt.MatchScore, &evt.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &evt, nil
}

// InsertEvent writes a new event.
func (r *Repository) InsertEvent(ctx context.Context, evt Event) (Event, error) {
	if evt.ID == "" {
		evt.ID = uuid.NewString()
	}
	if evt.When.IsZero() {
		evt.When = time.Now().UTC()
	}
	if evt.Status == "" {
		evt.Status = "pending"
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO attendance_events (id, user_id, device_id, occurred_at, location, image_url, status, match_score)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING created_at
	`, evt.ID, evt.UserID, evt.DeviceID, evt.When, evt.Location, evt.ImageURL, evt.Status, evt.MatchScore)
	if err := row.Scan(&evt.CreatedAt); err != nil {
		return Event{}, err
	}
	return evt, nil
}

// GetEvent returns a single event by id.
func (r *Repository) GetEvent(ctx context.Context, id string) (Event, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, device_id, occurred_at, location, image_url, status, match_score, created_at
		FROM attendance_events WHERE id = $1
	`, id)
	var evt Event
	if err := row.Scan(&evt.ID, &evt.UserID, &evt.DeviceID, &evt.When, &evt.Location, &evt.ImageURL, &evt.Status, &evt.MatchScore, &evt.CreatedAt); err != nil {
		return Event{}, err
	}
	return evt, nil
}

// UpdateEventStatus updates status and score after processing.
func (r *Repository) UpdateEventStatus(ctx context.Context, id, status string, score *float64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE attendance_events
		SET status = $2, match_score = COALESCE($3, match_score)
		WHERE id = $1
	`, id, status, score)
	return err
}

// ListEvents returns events with basic filters.
func (r *Repository) ListEvents(ctx context.Context, deviceID, userID string, limit, offset int) ([]Event, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT id, user_id, device_id, occurred_at, location, image_url, status, match_score, created_at FROM attendance_events`
	args := []any{}
	clauses := []string{}
	if deviceID != "" {
		clauses = append(clauses, "device_id = $"+itoa(len(args)+1))
		args = append(args, deviceID)
	}
	if userID != "" {
		clauses = append(clauses, "user_id = $"+itoa(len(args)+1))
		args = append(args, userID)
	}
	if len(clauses) > 0 {
		query += " WHERE " + joinClauses(clauses, " AND ")
	}
	query += " ORDER BY occurred_at DESC LIMIT $" + itoa(len(args)+1) + " OFFSET $" + itoa(len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Event
	for rows.Next() {
		var evt Event
		if err := rows.Scan(&evt.ID, &evt.UserID, &evt.DeviceID, &evt.When, &evt.Location, &evt.ImageURL, &evt.Status, &evt.MatchScore, &evt.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, evt)
	}
	return res, rows.Err()
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }

func joinClauses(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += sep + parts[i]
	}
	return out
}

// Employee represents a registered employee.
type Employee struct {
	ID           string     `json:"id"`
	EmployeeID   string     `json:"employee_id"`
	Name         *string    `json:"name,omitempty"`
	Email        *string    `json:"email,omitempty"`
	Department   *string    `json:"department,omitempty"`
	FaceEnrolled bool       `json:"face_enrolled"`
	EnrolledAt   *time.Time `json:"enrolled_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ListEmployees returns all employees.
func (r *Repository) ListEmployees(ctx context.Context) ([]Employee, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, employee_id, name, email, department, face_enrolled, enrolled_at, created_at
		FROM employees
		ORDER BY employee_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		if err := rows.Scan(&e.ID, &e.EmployeeID, &e.Name, &e.Email, &e.Department, &e.FaceEnrolled, &e.EnrolledAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		employees = append(employees, e)
	}
	return employees, rows.Err()
}

// GetEmployee returns a single employee by employee_id.
func (r *Repository) GetEmployee(ctx context.Context, employeeID string) (*Employee, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, employee_id, name, email, department, face_enrolled, enrolled_at, created_at
		FROM employees WHERE employee_id = $1
	`, employeeID)
	var e Employee
	if err := row.Scan(&e.ID, &e.EmployeeID, &e.Name, &e.Email, &e.Department, &e.FaceEnrolled, &e.EnrolledAt, &e.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

// UpsertEmployee creates or updates an employee.
func (r *Repository) UpsertEmployee(ctx context.Context, employeeID string, name *string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO employees (employee_id, name)
		VALUES ($1, $2)
		ON CONFLICT (employee_id) DO UPDATE SET
			name = COALESCE(EXCLUDED.name, employees.name),
			updated_at = NOW()
	`, employeeID, name)
	return err
}

// SetEmployeeFaceEnrolled marks an employee as face-enrolled.
func (r *Repository) SetEmployeeFaceEnrolled(ctx context.Context, employeeID string, enrolled bool) error {
	var enrolledAt interface{} = nil
	if enrolled {
		enrolledAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE employees
		SET face_enrolled = $2, enrolled_at = $3, updated_at = NOW()
		WHERE employee_id = $1
	`, employeeID, enrolled, enrolledAt)
	return err
}
