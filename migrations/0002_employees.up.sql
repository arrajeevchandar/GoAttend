-- Add employees table for face recognition enrollment
CREATE TABLE IF NOT EXISTS employees (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    employee_id TEXT NOT NULL UNIQUE,
    name TEXT,
    email TEXT,
    department TEXT,
    face_enrolled BOOLEAN NOT NULL DEFAULT FALSE,
    enrolled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_employees_employee_id ON employees(employee_id);
CREATE INDEX IF NOT EXISTS idx_employees_face_enrolled ON employees(face_enrolled);

-- Auto-create employees from existing attendance events
INSERT INTO employees (employee_id, face_enrolled, created_at)
SELECT DISTINCT user_id, FALSE, MIN(occurred_at)
FROM attendance_events
WHERE user_id NOT IN (SELECT employee_id FROM employees)
GROUP BY user_id
ON CONFLICT (employee_id) DO NOTHING;
