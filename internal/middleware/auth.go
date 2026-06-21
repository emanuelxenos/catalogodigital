package middleware

import (
	"context"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

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

			// Valida se o plano do lojista está expirado (se não for plano bronze/grátis ID 1)
			if shop != nil && shop.PlanID != 1 {
				if shop.PlanExpiresAt != nil && time.Now().After(*shop.PlanExpiresAt) {
					path := r.URL.Path
					// Permite apenas rotas de fatura/plano e logout
					isBillingRoute := path == "/admin/plano" || path == "/admin/plano/faturas" ||
						strings.HasPrefix(path, "/admin/plano/faturas/") || path == "/admin/plano/upgrade" ||
						path == "/admin/plano/upgrade/cartao" || strings.HasPrefix(path, "/admin/plano/status/") ||
						path == "/admin/logout"
					if !isBillingRoute {
						http.Redirect(w, r, "/admin/plano?expired=true", http.StatusSeeOther)
						return
					}
				}
			}

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

// MaintenanceMode verifica se o sistema está em manutenção e bloqueia requisições públicas/admin
func MaintenanceMode(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			// Ignora caminhos do master admin, arquivos estáticos e favicon
			if strings.HasPrefix(path, "/master") || path == "/favicon.ico" || strings.HasPrefix(path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			// Busca configurações globais do banco
			configs, err := db.GetPlatformConfigs(r.Context())
			if err == nil && configs["maintenance_mode"] == "true" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusServiceUnavailable)

				whatsapp := configs["support_whatsapp"]
				data := map[string]interface{}{
					"SupportWhatsApp": whatsapp,
				}

				tmpl, err := template.ParseFiles(filepath.Join("templates", "maintenance.html"))
				if err != nil {
					w.Write([]byte(`<h1>Modo de Manutencao</h1><p>A plataforma esta em manutencao programada. Voltaremos em breve!</p>`))
					return
				}
				_ = tmpl.Execute(w, data)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

