package initializer

import (
	"log"

	"github.com/joho/godotenv"
)

func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Error loading .env: %v\n", err)
	}
}
