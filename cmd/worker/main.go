package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"attendance/internal/attendance"
	"attendance/internal/config"
	"attendance/internal/faceclient"
	"attendance/internal/queue"
	"attendance/internal/store"
)

// Worker consumes queue messages, calls face service, and updates events.
func main() {
	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutdown signal received")
		cancel()
	}()

	db, err := store.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	defer db.Close()

	redisClient := store.NewRedis(cfg.RedisAddr)

	var q queue.Queue
	if cfg.QueueBackend == "memory" {
		q = queue.NewInMemory(64)
	} else {
		q = queue.NewRedisQueue(redisClient.Client, "attendance:checkins")
	}

	repo := attendance.NewRepository(db.Client)
	face := faceclient.New(cfg.FaceServiceURL, cfg.FaceSkip)

	// Check face service health on startup
	if !cfg.FaceSkip {
		if err := face.Health(ctx); err != nil {
			log.Printf("WARNING: Face service not available: %v", err)
			log.Println("Worker will retry face processing when events arrive")
		} else {
			log.Println("Face service connected")
		}
	}

	messages, err := q.Consume(ctx)
	if err != nil {
		log.Fatalf("queue consume init failed: %v", err)
	}

	log.Println("worker started, waiting for messages...")
	for msg := range messages {
		if msg.Type != "checkin" {
			continue
		}

		id := string(msg.Body)
		log.Printf("processing event %s", id)

		evt, err := repo.GetEvent(ctx, id)
		if err != nil {
			log.Printf("fetch event %s failed: %v", id, err)
			continue
		}

		// Call face service to get embedding and score
		result, err := face.EmbedWithScore(ctx, evt.ImageURL)
		if err != nil {
			log.Printf("face embed failed for %s: %v", id, err)
			_ = repo.UpdateEventStatus(ctx, id, "failed", nil)
			continue
		}

		// Use actual detection confidence from face service
		score := result.Score
		log.Printf("event %s: detected %d face(s), confidence: %.2f", id, result.FacesDetected, score)

		// Mark as processed with the face detection score
		_ = repo.UpdateEventStatus(ctx, id, "processed", &score)
		log.Printf("event %s processed successfully", id)

		time.Sleep(10 * time.Millisecond) // Small delay between processing
	}

	log.Println("worker stopped")
}
