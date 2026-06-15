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

// RequireSuperAdmin garante que o usuário autenticado é um Super Admin da plataforma
func RequireSuperAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r)
			if user == nil || !user.IsSuperAdmin {
				http.Error(w, "Acesso Negado: Área restrita ao administrador do sistema", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireMasterAuth garante que o administrador mestre está logado via cookie exclusivo do SaaS
func RequireMasterAuth(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("master_session_id")
			if err != nil {
				http.Redirect(w, r, "/master/login", http.StatusSeeOther)
				return
			}

			session, err := db.GetSession(r.Context(), cookie.Value)
			if err != nil {
				// Limpa o cookie inválido
				http.SetCookie(w, &http.Cookie{
					Name:     "master_session_id",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
				})
				http.Redirect(w, r, "/master/login", http.StatusSeeOther)
				return
			}

			user, err := db.GetUserByID(r.Context(), session.UserID)
			if err != nil || !user.IsSuperAdmin {
				// Usuário inválido ou não autorizado
				http.SetCookie(w, &http.Cookie{
					Name:     "master_session_id",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
				})
				http.Redirect(w, r, "/master/login", http.StatusSeeOther)
				return
			}

			// Injeta user no contexto
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

