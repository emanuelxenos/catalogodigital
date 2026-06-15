package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dbURL := "postgres://postgres:admin123@localhost:5432/catalogo?sslmode=disable"
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Erro ao conectar ao DB: %v", err)
	}
	defer pool.Close()

	email := "admin@admin.com"
	password := "admin123"

	// Garante que a coluna is_super_admin exista
	_, err = pool.Exec(context.Background(), "ALTER TABLE users ADD COLUMN IF NOT EXISTS is_super_admin BOOLEAN DEFAULT FALSE")
	if err != nil {
		log.Fatalf("Erro ao criar coluna is_super_admin: %v", err)
	}

	var userID int
	err = pool.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err == nil {
		fmt.Printf("Usuário %s já existe com ID %d. Garantindo que seja Super Admin...\n", email, userID)
		_, err = pool.Exec(context.Background(), "UPDATE users SET is_super_admin = TRUE WHERE id = $1", userID)
		if err != nil {
			log.Fatalf("Erro ao promover a Super Admin: %v", err)
		}
		fmt.Println("Super Admin promovido com sucesso!")
		return
	}

	// Se não existe, cria
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Erro ao gerar hash: %v", err)
	}

	err = pool.QueryRow(context.Background(), 
		"INSERT INTO users (name, email, password_hash, is_super_admin) VALUES ($1, $2, $3, $4) RETURNING id",
		"Administrador", email, string(hash), true).Scan(&userID)
	if err != nil {
		log.Fatalf("Erro ao inserir usuário: %v", err)
	}
	fmt.Printf("Usuário Super Admin %s criado com sucesso! ID: %d\n", email, userID)
}
