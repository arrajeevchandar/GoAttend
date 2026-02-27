package handler

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/darshan/goattend/internal/cloudinary"
	"github.com/darshan/goattend/internal/faceclient"
	"github.com/darshan/goattend/internal/model"
	"github.com/darshan/goattend/internal/store"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	store      *store.Store
	cloud      *cloudinary.Client // nil if Cloudinary not configured
	faceClient *faceclient.Client
}

func New(s *store.Store, cloud *cloudinary.Client, faceClient *faceclient.Client) *Handler {
	return &Handler{store: s, cloud: cloud, faceClient: faceClient}
}

// ---------- Health ----------

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ---------- Register Student ----------

type registerRequest struct {
	Name       string `form:"name" binding:"required"`
	Email      string `form:"email" binding:"required,email"`
	StudentID  string `form:"student_id" binding:"required"`
	Department string `form:"department"`
}

// RegisterStudent handles registration: saves student info + uploads photo to Cloudinary + registers face.
// Expects multipart form with fields: name, email, student_id, department, photo (file).
func (h *Handler) RegisterStudent(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Read photo file
	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "photo file is required"})
		return
	}
	defer file.Close()

	// Read photo bytes into memory (needed for both Cloudinary and face service)
	photoBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read photo"})
		return
	}

	// 1. Upload photo to Cloudinary
	var photoURL string
	if h.cloud != nil {
		result, err := h.cloud.Upload(bytes.NewReader(photoBytes), header.Filename, "goattend/students")
		if err != nil {
			log.Printf("cloudinary upload error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload photo"})
			return
		}
		photoURL = result.SecureURL
	}

	// 2. Save student to DB
	st := &model.Student{
		Name:       req.Name,
		Email:      req.Email,
		StudentID:  req.StudentID,
		Department: req.Department,
		PhotoURL:   photoURL,
	}
	if err := h.store.CreateStudent(st); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "student already exists: " + err.Error()})
		return
	}

	// 3. Register face with face service (using DB id as identifier)
	if h.faceClient != nil {
		_, err := h.faceClient.Register(st.ID, bytes.NewReader(photoBytes), header.Filename)
		if err != nil {
			log.Printf("face service register error: %v", err)
			// Don't fail registration — face can be re-registered later
		}
	}

	c.JSON(http.StatusCreated, st)
}

// ---------- Login via Face (= Mark Attendance) ----------

// FaceLogin accepts a photo, sends it to face service for recognition,
// and if matched, marks attendance for that student.
func (h *Handler) FaceLogin(c *gin.Context) {
	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "photo file is required"})
		return
	}
	defer file.Close()

	photoBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read photo"})
		return
	}

	// Call face service to recognize
	if h.faceClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "face service not configured"})
		return
	}

	result, err := h.faceClient.Recognize(bytes.NewReader(photoBytes), header.Filename)
	if err != nil {
		log.Printf("face recognize error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "face recognition failed"})
		return
	}

	if !result.Matched {
		c.JSON(http.StatusUnauthorized, gin.H{
			"matched": false,
			"error":   "face not recognized",
		})
		return
	}

	// Get student info
	student, err := h.store.GetStudentByID(result.StudentID)
	if err != nil || student == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "matched student not found in database"})
		return
	}

	// Mark attendance
	rec, err := h.store.MarkAttendance(student.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark attendance"})
		return
	}

	rec.Name = student.Name
	c.JSON(http.StatusOK, gin.H{
		"matched":    true,
		"student":    student,
		"attendance": rec,
	})
}

// ---------- List Endpoints ----------

func (h *Handler) ListStudents(c *gin.Context) {
	students, err := h.store.ListStudents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if students == nil {
		students = []model.Student{}
	}
	c.JSON(http.StatusOK, students)
}

func (h *Handler) GetStudent(c *gin.Context) {
	id := c.Param("id")
	student, err := h.store.GetStudentByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if student == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "student not found"})
		return
	}
	c.JSON(http.StatusOK, student)
}

func (h *Handler) ListAttendance(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	records, err := h.store.ListAttendance(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if records == nil {
		records = []model.AttendanceRecord{}
	}
	c.JSON(http.StatusOK, records)
}
