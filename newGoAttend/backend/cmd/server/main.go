package main

import (
	"log"
	"net/http"

	cld "github.com/darshan/goattend/internal/cloudinary"
	"github.com/darshan/goattend/internal/config"
	"github.com/darshan/goattend/internal/faceclient"
	"github.com/darshan/goattend/internal/handler"
	"github.com/darshan/goattend/internal/store"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// Database
	db, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	// Cloudinary (optional)
	var cloud *cld.Client
	if cfg.CloudinaryURL != "" {
		cloud, err = cld.New(cfg.CloudinaryURL)
		if err != nil {
			log.Printf("WARNING: cloudinary disabled: %v", err)
		} else {
			log.Println("Cloudinary configured")
		}
	} else {
		log.Println("WARNING: CLOUDINARY_URL not set, photo uploads disabled")
	}

	// Face service client
	fc := faceclient.New(cfg.FaceServiceURL)
	log.Printf("Face service: %s", cfg.FaceServiceURL)

	h := handler.New(db, cloud, fc)

	// Router
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		AllowCredentials: true,
	}))

	// Serve frontend
	r.Static("/static", cfg.FrontendDir)
	r.StaticFile("/", cfg.FrontendDir+"/index.html")
	r.StaticFile("/register", cfg.FrontendDir+"/pages/register.html")
	r.StaticFile("/attendance", cfg.FrontendDir+"/pages/attendance.html")
	r.StaticFile("/students", cfg.FrontendDir+"/pages/students.html")

	// API routes
	api := r.Group("/api")
	{
		api.GET("/healthz", h.Healthz)

		// Register student (multipart: name, email, student_id, department, photo)
		api.POST("/students", h.RegisterStudent)
		api.GET("/students", h.ListStudents)
		api.GET("/students/:id", h.GetStudent)

		// Face login = mark attendance
		api.POST("/face-login", h.FaceLogin)
		api.GET("/attendance", h.ListAttendance)
	}

	r.NoRoute(func(c *gin.Context) {
		c.File(cfg.FrontendDir + "/index.html")
	})

	log.Printf("Server starting on : http://localhost:%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
