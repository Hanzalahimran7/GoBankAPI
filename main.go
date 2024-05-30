package main

import (
	"log"
)

func main() {
	store, err := NewPostgresStore(Config{DBName: "hanzalah", User: "hanzalah", Password: "hanzalah123", Port: "5432", SSLMode: "disable", Host: "localhost"})
	if err != nil {
		log.Fatal("Cannot connect to DB")
	}
	server := NewAPIServer(":3000", store)
	server.Run()
}
