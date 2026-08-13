package main

import (
	"log"

	"github.com/anurag46-code/workflow-engine/internal/api"
	"github.com/anurag46-code/workflow-engine/internal/engine"
	"github.com/anurag46-code/workflow-engine/internal/store"
	"github.com/anurag46-code/workflow-engine/internal/worker"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	s, err := store.New()
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer s.Close()

	if err := s.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("database ready")

	eng := engine.New(s)

	// Start worker pool - 3 concurrent workers polling the Postgres queue
	for i := 0; i < 3; i++ {
		w := worker.New(s, eng)
		go w.Run()
	}
	log.Println("3 workers started")

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:3001", "http://localhost", "http://15.252.165.209:3001", "http://15.252.165.209"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		AllowCredentials: true,
	}))

	h := api.New(s, eng)
	h.RegisterRoutes(r)

	log.Println("server starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("server: %v", err)
	}
}
