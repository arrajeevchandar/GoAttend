package config

import "os"

type Config struct {
	Port        string
	DBPath      string
	UploadDir   string
	FrontendDir string

	// Cloudinary
	CloudinaryURL string // CLOUDINARY_URL=cloudinary://key:secret@cloud_name

	// Face service
	FaceServiceURL string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		DBPath:         getEnv("DB_PATH", "./goattend.db"),
		UploadDir:      getEnv("UPLOAD_DIR", "./uploads"),
		FrontendDir:    getEnv("FRONTEND_DIR", "../frontend"),
		CloudinaryURL:  getEnv("CLOUDINARY_URL", ""),
		FaceServiceURL: getEnv("FACE_SERVICE_URL", "http://localhost:8000"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
