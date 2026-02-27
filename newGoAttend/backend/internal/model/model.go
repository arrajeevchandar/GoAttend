package model

import "time"

// Student represents a registered student.
type Student struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	StudentID  string    `json:"student_id"`
	Department string    `json:"department"`
	PhotoURL   string    `json:"photo_url,omitempty"` // Cloudinary URL
	CreatedAt  time.Time `json:"created_at"`
}

// AttendanceRecord represents a single attendance log entry.
type AttendanceRecord struct {
	ID        string    `json:"id"`
	StudentID string    `json:"student_id"`
	Name      string    `json:"name,omitempty"` // joined from students
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // "present"
}
