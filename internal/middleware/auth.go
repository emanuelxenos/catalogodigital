package middleware

import (
	"context"
	"net/http"

	"catalogo/internal/database"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
	ShopContextKey contextKey = "shop"
)

// RequireAuth verifica se o usuário está autenticado via cookie de sessão
func RequireAuth(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session_id")
			if err != nil {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			session, err := db.GetSession(r.Context(), cookie.Value)
			if err != nil {
				// Sessão inválida ou expirada: limpa o cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "session_id",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
				})
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			// Busca o usuário da sessão
			user, err := db.GetUserByID(r.Context(), session.UserID)
			if err != nil {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			// Busca a loja do usuário
			shop, err := db.GetShopByUserID(r.Context(), user.ID)
			if err != nil {
				// Usuário sem loja - poderia redirecionar para criação
				// Por enquanto, permite acesso sem loja
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Injeta user e shop no contexto
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			ctx = context.WithValue(ctx, ShopContextKey, shop)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext extrai o usuário do contexto da requisição
func GetUserFromContext(r *http.Request) *database.User {
	user, ok := r.Context().Value(UserContextKey).(*database.User)
	if !ok {
		return nil
	}
	return user
}

// GetShopFromContext extrai a loja do contexto da requisição
func GetShopFromContext(r *http.Request) *database.Shop {
	shop, ok := r.Context().Value(ShopContextKey).(*database.Shop)
	if !ok {
		return nil
	}
	return shop
}
