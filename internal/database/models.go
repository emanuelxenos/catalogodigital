package database

import (
	"time"
)

// User representa um lojista/administrador
type User struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Shop representa uma loja (tenant)
type Shop struct {
	ID             int       `json:"id"`
	UserID         int       `json:"user_id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	WhatsappNumber string    `json:"whatsapp_number"`
	LogoURL        string    `json:"logo_url"`
	PrimaryColor   string    `json:"primary_color"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

// Category representa uma categoria de produtos
type Category struct {
	ID        int       `json:"id"`
	ShopID    int       `json:"shop_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// Product representa um produto do catálogo
type Product struct {
	ID           int       `json:"id"`
	ShopID       int       `json:"shop_id"`
	CategoryID   *int      `json:"category_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Price        float64   `json:"price"`
	ImageURL     string    `json:"image_url"`
	IsAvailable  bool      `json:"is_available"`
	CreatedAt    time.Time `json:"created_at"`
	CategoryName string    `json:"category_name,omitempty"` // JOIN auxiliar
}

// Session representa uma sessão de autenticação
type Session struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
