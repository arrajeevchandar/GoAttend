# GoAttend — Face Recognition Attendance System

A face-recognition-powered attendance system built with a **Go backend**, **Python face-recognition microservice** (DeepFace), and a **vanilla HTML/CSS/JS frontend**.

---

## Architecture

```
┌──────────────┐       ┌──────────────────┐       ┌──────────────────┐
│   Frontend   │──────▶│   Go Backend     │──────▶│  Face Service    │
│  (HTML/JS)   │       │  (Gin + SQLite)  │       │  (FastAPI/Deep-  │
│  Port 8080   │       │  Port 8080       │       │   Face) Port 8000│
└──────────────┘       └──────────────────┘       └──────────────────┘
                              │
                              ▼
                       ┌──────────────┐
                       │   SQLite DB  │
                       │ goattend.db  │
                       └──────────────┘
```

**Flow:**
1. **Register** — Student submits name, email, ID, department, and a photo.
   - Photo is uploaded to Cloudinary (optional).
   - Student record is saved in SQLite.
   - Photo is sent to the Face Service to register the face.
2. **Attendance (Face Login)** — Student submits a live photo.
   - Photo is sent to the Face Service for recognition.
   - If a match is found, attendance is marked in SQLite.

---

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| **Go** | 1.23+ | Backend server |
| **Python** | 3.12 | Face recognition service |
| **GCC / C compiler** | Any | Required for `go-sqlite3` (CGo) |
| **pip** | Latest | Python dependency management |

---

## Project Structure

```
newGoAttend/
├── backend/                   # Go backend
│   ├── cmd/server/main.go     # Entry point
│   ├── internal/
│   │   ├── config/            # Environment-based configuration
│   │   ├── cloudinary/        # Cloudinary image upload client
│   │   ├── faceclient/        # HTTP client for the face service
│   │   ├── handler/           # Gin HTTP handlers
│   │   ├── middleware/        # Middleware (CORS, etc.)
│   │   ├── model/             # Data models (Student, Attendance)
│   │   └── store/             # SQLite data access layer
│   ├── go.mod
│   └── go.sum
├── face-service/              # Python face recognition microservice
│   ├── main.py                # FastAPI app
│   ├── requirements.txt
│   ├── faces/                 # Registered face images (<db_id>.jpg)
│   └── venv/                  # Python virtual environment
├── frontend/                  # Vanilla HTML/CSS/JS frontend
│   ├── index.html
│   ├── css/
│   ├── js/
│   └── pages/
│       ├── register.html
│       ├── attendance.html
│       └── students.html
└── README.md
```

---

## Setup & Execution

### 1. Face Service (Python) — Start First

The face service must be running before the backend starts, since the backend calls it for face registration and recognition.

```bash
cd face-service

# Create virtual environment (skip if venv/ already exists)
python3 -m venv venv

# Activate virtual environment
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Run the service
python main.py
```

The face service will start on **`http://localhost:8000`** by default.

> **First run note:** DeepFace will download the VGG-Face model (~580 MB) on the first face operation. This is a one-time download.

#### Face Service Environment Variables (optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8000` | Port to run the face service on |
| `FACES_DIR` | `./faces` | Directory to store registered face images |
| `MODEL_NAME` | `VGG-Face` | DeepFace model (`VGG-Face`, `Facenet`, `ArcFace`, etc.) |
| `DETECTOR` | `opencv` | Face detector backend (`opencv`, `retinaface`, `mtcnn`) |
| `DISTANCE_METRIC` | `cosine` | Similarity metric (`cosine`, `euclidean`, `euclidean_l2`) |
| `THRESHOLD` | `0.40` | Max distance to consider a match (lower = stricter) |

#### Verify it's running

```bash
curl http://localhost:8000/health
# {"status":"ok","registered_faces":0,"model":"VGG-Face"}
```

---

### 2. Go Backend — Start Second

```bash
cd backend

# Download Go dependencies
go mod download

# Build the server
CGO_ENABLED=1 go build -o server ./cmd/server

# Run the server
./server
```

Or run directly without building:

```bash
cd backend
CGO_ENABLED=1 go run ./cmd/server
```

The backend will start on **`http://localhost:8080`** by default.

> **Note:** `CGO_ENABLED=1` is required because the SQLite driver (`go-sqlite3`) uses CGo. Make sure you have GCC installed.

#### Backend Environment Variables (optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Port for the backend server |
| `DB_PATH` | `./goattend.db` | SQLite database file path |
| `UPLOAD_DIR` | `./uploads` | Local upload directory |
| `FRONTEND_DIR` | `../frontend` | Path to the frontend static files |
| `CLOUDINARY_URL` | *(empty)* | Cloudinary URL (`cloudinary://KEY:SECRET@CLOUD_NAME`). If not set, photo upload to cloud is skipped. |
| `FACE_SERVICE_URL` | `http://localhost:8000` | URL of the face recognition microservice |

#### Verify it's running

```bash
curl http://localhost:8080/api/healthz
# {"status":"ok"}
```

---

### 3. Frontend

The frontend is served automatically by the Go backend as static files. Once the backend is running, open your browser:

```
http://localhost:8080
```

**Pages:**
- `/` — Home / Dashboard
- `/register` — Register a new student (with face photo)
- `/attendance` — Mark attendance via face recognition
- `/students` — View registered students

---

## Quick Start (TL;DR)

Open **three terminals** and run:

```bash
# Terminal 1 — Face Service
cd face-service && source venv/bin/activate && python main.py

# Terminal 2 — Backend
cd backend && CGO_ENABLED=1 go run ./cmd/server

# Terminal 3 — Open in browser
xdg-open http://localhost:8080   # Linux
# or: open http://localhost:8080  # macOS
```

---

## API Reference

All API endpoints are prefixed with `/api`.

### Health Check
```
GET /api/healthz
```

### Students

| Method | Endpoint | Description | Body |
|--------|----------|-------------|------|
| `POST` | `/api/students` | Register a new student | Multipart form: `name`, `email`, `student_id`, `department`, `photo` (file) |
| `GET` | `/api/students` | List all students | — |
| `GET` | `/api/students/:id` | Get a student by DB ID | — |

### Attendance

| Method | Endpoint | Description | Body |
|--------|----------|-------------|------|
| `POST` | `/api/face-login` | Recognize face & mark attendance | Multipart form: `photo` (file) |
| `GET` | `/api/attendance` | List attendance records | Query: `?limit=50` |

---

## Cloudinary Setup (Optional)

If you want student photos stored in the cloud:

1. Create a free [Cloudinary](https://cloudinary.com) account.
2. In your Cloudinary dashboard, go to **Settings → Upload** and create an **unsigned upload preset** named `goattend`.
3. Set the `CLOUDINARY_URL` environment variable:
   ```bash
   export CLOUDINARY_URL="cloudinary://API_KEY:API_SECRET@CLOUD_NAME"
   ```
4. Restart the backend.

If `CLOUDINARY_URL` is not set, the system will still work — photos are sent to the face service for recognition but won't be stored in the cloud.

---

