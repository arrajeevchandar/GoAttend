package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/darshan/goattend/internal/model"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	if dir := filepath.Dir(dbPath); dir != "." {
		os.MkdirAll(dir, 0o755)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS students (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		email       TEXT UNIQUE NOT NULL,
		student_id  TEXT UNIQUE NOT NULL,
		department  TEXT NOT NULL DEFAULT '',
		photo_url   TEXT NOT NULL DEFAULT '',
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS attendance (
		id          TEXT PRIMARY KEY,
		student_id  TEXT NOT NULL REFERENCES students(id),
		timestamp   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		status      TEXT NOT NULL DEFAULT 'present'
	);

	CREATE INDEX IF NOT EXISTS idx_attendance_student ON attendance(student_id);
	CREATE INDEX IF NOT EXISTS idx_attendance_time    ON attendance(timestamp);
	`
	_, err := db.Exec(schema)
	return err
}

func (s *Store) Close() error { return s.db.Close() }

// -------- Students --------

func (s *Store) CreateStudent(st *model.Student) error {
	st.ID = uuid.New().String()
	st.CreatedAt = time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO students (id, name, email, student_id, department, photo_url, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		st.ID, st.Name, st.Email, st.StudentID, st.Department, st.PhotoURL, st.CreatedAt,
	)
	return err
}

func (s *Store) ListStudents() ([]model.Student, error) {
	rows, err := s.db.Query(`SELECT id, name, email, student_id, department, photo_url, created_at FROM students ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var students []model.Student
	for rows.Next() {
		var st model.Student
		if err := rows.Scan(&st.ID, &st.Name, &st.Email, &st.StudentID, &st.Department, &st.PhotoURL, &st.CreatedAt); err != nil {
			return nil, err
		}
		students = append(students, st)
	}
	return students, rows.Err()
}

func (s *Store) GetStudentByID(id string) (*model.Student, error) {
	var st model.Student
	err := s.db.QueryRow(
		`SELECT id, name, email, student_id, department, photo_url, created_at FROM students WHERE id = ?`, id,
	).Scan(&st.ID, &st.Name, &st.Email, &st.StudentID, &st.Department, &st.PhotoURL, &st.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &st, err
}

func (s *Store) UpdateStudentPhoto(id, photoURL string) error {
	_, err := s.db.Exec(`UPDATE students SET photo_url = ? WHERE id = ?`, photoURL, id)
	return err
}

// -------- Attendance --------

func (s *Store) MarkAttendance(studentID string) (*model.AttendanceRecord, error) {
	rec := &model.AttendanceRecord{
		ID:        uuid.New().String(),
		StudentID: studentID,
		Timestamp: time.Now().UTC(),
		Status:    "present",
	}
	_, err := s.db.Exec(
		`INSERT INTO attendance (id, student_id, timestamp, status) VALUES (?, ?, ?, ?)`,
		rec.ID, rec.StudentID, rec.Timestamp, rec.Status,
	)
	return rec, err
}

func (s *Store) ListAttendance(limit int) ([]model.AttendanceRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT a.id, a.student_id, s.name, a.timestamp, a.status
		 FROM attendance a
		 JOIN students s ON s.id = a.student_id
		 ORDER BY a.timestamp DESC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []model.AttendanceRecord
	for rows.Next() {
		var r model.AttendanceRecord
		if err := rows.Scan(&r.ID, &r.StudentID, &r.Name, &r.Timestamp, &r.Status); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}
