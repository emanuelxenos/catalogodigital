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

	email := "manuenbs@gmail.com"
	password := "admin123"

	var userID int
	err = pool.QueryRow(context.Background(), "SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err == nil {
		fmt.Printf("Usuário %s já existe com ID %d\n", email, userID)
		// Verifica se tem loja
		var shopID int
		var shopName string
		err = pool.QueryRow(context.Background(), "SELECT id, name FROM shops WHERE user_id = $1", userID).Scan(&shopID, &shopName)
		if err == nil {
			fmt.Printf("Loja associada encontrada: ID %d, Nome: %s\n", shopID, shopName)
		} else {
			fmt.Printf("Nenhuma loja associada para o usuário %d. Criando uma loja default...\n", userID)
			_, err = pool.Exec(context.Background(), 
				"INSERT INTO shops (user_id, name, slug, whatsapp_number, primary_color) VALUES ($1, $2, $3, $4, $5)",
				userID, "Minha Loja", "minha-loja", "5511999999999", "#8B5CF6")
			if err != nil {
				log.Fatalf("Erro ao criar loja: %v", err)
			}
			fmt.Println("Loja criada com sucesso!")
		}
		return
	}

	// Se não existe, cria
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Erro ao gerar hash: %v", err)
	}

	err = pool.QueryRow(context.Background(), 
		"INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id",
		"Emanuel", email, string(hash)).Scan(&userID)
	if err != nil {
		log.Fatalf("Erro ao inserir usuário: %v", err)
	}
	fmt.Printf("Usuário %s criado com sucesso! ID: %d\n", email, userID)

	// Cria loja default
	_, err = pool.Exec(context.Background(), 
		"INSERT INTO shops (user_id, name, slug, whatsapp_number, primary_color) VALUES ($1, $2, $3, $4, $5)",
		userID, "Minha Loja", "minha-loja", "5511999999999", "#8B5CF6")
	if err != nil {
		log.Fatalf("Erro ao criar loja: %v", err)
	}
	fmt.Println("Loja default criada com sucesso!")
}
