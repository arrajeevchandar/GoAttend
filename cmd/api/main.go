package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"attendance/internal/attendance"
	"attendance/internal/auth"
	"attendance/internal/cloudinary"
	"attendance/internal/config"
	"attendance/internal/faceclient"
	"attendance/internal/httpmiddleware"
	"attendance/internal/queue"
	"attendance/internal/store"
)

func main() {
	cfg := config.Load()

	// Set Gin mode based on environment
	if cfg.Env == "production" || cfg.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	if err := runHTTP(cfg); err != nil {
		log.Fatalf("http server failed: %v", err)
	}
}

func runHTTP(cfg config.App) error {
	db, err := store.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Printf("warning: db not reachable: %v", err)
	}
	defer func() {
		if db != nil {
			_ = db.Close()
		}
	}()

	redisClient := store.NewRedis(cfg.RedisAddr)
	_ = faceclient.New(cfg.FaceServiceURL, cfg.FaceSkip) // used by worker, included for compile check

	var q queue.Queue
	if cfg.QueueBackend == "memory" {
		q = queue.NewInMemory(64)
	} else {
		q = queue.NewRedisQueue(redisClient.Client, "attendance:checkins")
	}

	repo := attendance.NewRepository(db.Client)
	att := attendance.NewService(repo, 5*time.Minute)
	ctx := context.Background()

	// Cloudinary client (nil when not configured)
	var cdnClient *cloudinary.Client
	if cfg.CloudinaryCloudName != "" && cfg.CloudinaryAPIKey != "" && cfg.CloudinaryAPISecret != "" {
		cdnClient = cloudinary.New(cfg.CloudinaryCloudName, cfg.CloudinaryAPIKey, cfg.CloudinaryAPISecret, cfg.CloudinaryFolder)
		log.Println("Cloudinary configured:", cfg.CloudinaryCloudName)
	} else {
		log.Println("Cloudinary not configured (CLOUDINARY_CLOUD_NAME / API_KEY / API_SECRET not set)")
	}

	r := gin.New()
	
	// Recovery middleware
	r.Use(gin.Recovery())
	
	// Custom logger
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/healthz", "/metrics"},
	}))
	
	// CORS middleware
	r.Use(corsMiddleware())
	
	// Security headers
	r.Use(securityHeaders())
	
	// Rate limiting
	r.Use(httpmiddleware.NewSimpleTokenBucket(cfg.RateLimitPerMin, cfg.RateLimitPerMin).GinMiddleware())

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/healthz", func(c *gin.Context) {
		redisHealthy := redisClient.Healthy(c.Request.Context())
		dbHealthy := db != nil
		status := http.StatusOK
		if !redisHealthy || !dbHealthy {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"status": "ok", "redis": redisHealthy, "db": dbHealthy})
	})

	r.POST("/v1/devices/register", func(c *gin.Context) {
		var req struct {
			DeviceID string `json:"device_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := att.RegisterDevice(c.Request.Context(), req.DeviceID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		tokens, err := auth.Issue(req.DeviceID, "device", cfg.JWTIssuer, cfg.JWTSigningKey, cfg.AccessTTL, cfg.RefreshTTL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "token issue failed"})
			return
		}

		_ = repo.SaveRefreshToken(c.Request.Context(), req.DeviceID, tokens.RefreshToken, tokens.RefreshExp)

		c.JSON(http.StatusCreated, gin.H{
			"access_token":  tokens.AccessToken,
			"refresh_token": tokens.RefreshToken,
			"expires_at":    tokens.AccessExp.Unix(),
		})
	})

	// Upload endpoint — uploads a base64 image or multipart file to Cloudinary
	// Returns the public Cloudinary URL so the caller can use it in /v1/checkins
	authGroup := r.Group("/v1", auth.DeviceAuth(cfg.JWTSigningKey, cfg.JWTIssuer))

	authGroup.POST("/upload", func(c *gin.Context) {
		if cdnClient == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "image storage not configured"})
			return
		}

		contentType := c.ContentType()
		var result *cloudinary.UploadResult
		var err error

		switch {
		case strings.Contains(contentType, "multipart/form-data"):
			// Multipart file upload
			file, header, ferr := c.Request.FormFile("file")
			if ferr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "file field required"})
				return
			}
			defer file.Close()
			data, ferr := io.ReadAll(file)
			if ferr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "read file failed"})
				return
			}
			result, err = cdnClient.UploadBytes(data, header.Filename)

		default:
			// JSON body with base64 data URL
			var body struct {
				Data string `json:"data" binding:"required"`
			}
			if berr := c.ShouldBindJSON(&body); berr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "provide {\"data\": \"<base64 data URL>\"}"})
				return
			}
			result, err = cdnClient.UploadBase64(body.Data)
		}

		if err != nil {
			log.Printf("cloudinary upload failed: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "image upload failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"url":       result.SecureURL,
			"public_id": result.PublicID,
			"width":     result.Width,
			"height":    result.Height,
			"bytes":     result.Bytes,
		})
	})

	authGroup.POST("/checkins", func(c *gin.Context) {
		var req struct {
			UserID   string `json:"user_id" binding:"required"`
			DeviceID string `json:"device_id" binding:"required"`
			Location string `json:"location"`
			ImageURL string `json:"image_url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		claimsAny, _ := c.Get("claims")
		claims, _ := claimsAny.(auth.Claims)
		if claims.Subject != "" && claims.Subject != req.DeviceID {
			c.JSON(http.StatusForbidden, gin.H{"error": "device mismatch"})
			return
		}

		evt, err := att.CheckIn(c.Request.Context(), req.UserID, req.DeviceID, req.Location, req.ImageURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := q.Publish(ctx, queue.Message{Type: "checkin", Body: []byte(evt.ID)}); err != nil {
			log.Printf("queue publish failed: %v", err)
		}

		c.JSON(http.StatusAccepted, gin.H{"event_id": evt.ID, "when": evt.When, "status": evt.Status})
	})

	authGroup.GET("/events", func(c *gin.Context) {
		deviceID := c.Query("device_id")
		userID := c.Query("user_id")
		limit, offset := 50, 0
		if v := c.Query("limit"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				limit = parsed
			}
		}
		if v := c.Query("offset"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				offset = parsed
			}
		}
		events, err := repo.ListEvents(c.Request.Context(), deviceID, userID, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"events": events})
	})

	r.StaticFile("/", "web/index.html")
	r.Static("/static", "web/static")

	// Graceful shutdown
	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on :%s", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give outstanding requests 10 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced shutdown: %v", err)
	}

	log.Println("Server exited")
	return nil
}

// CORS middleware for browser requests
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Security headers middleware
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Only add HSTS in production
		if gin.Mode() == gin.ReleaseMode {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		
		c.Next()
	}
}

// ensure imports are used when stubbing; this avoids lints when DB unused.
func init() {
	_, _ = os.LookupEnv("APP_ENV")
}
