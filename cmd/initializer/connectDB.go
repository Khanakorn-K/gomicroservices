package initializer

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectDB() *pgxpool.Pool {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Print(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	return pool
}
