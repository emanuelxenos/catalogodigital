package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"catalogo/internal/asaas"
	"catalogo/internal/database"
	"catalogo/internal/mail"
)

// TemplateEngine gerencia o carregamento e cache de templates
type TemplateEngine struct {
	templates map[string]*template.Template
	mu        sync.RWMutex
	baseDir   string
	funcMap   template.FuncMap
	devMode   bool
}

// Handlers encapsula as dependências compartilhadas entre todos os handlers
type Handlers struct {
	DB          *database.DB
	Tmpl        *TemplateEngine
	Mailer      *mail.Mailer
	AsaasClient *asaas.Client
}

// NewHandlers cria uma nova instância de Handlers
func NewHandlers(db *database.DB, mailer *mail.Mailer, devMode bool, asaasClient *asaas.Client) *Handlers {
	tmpl := NewTemplateEngine("templates", devMode)
	return &Handlers{
		DB:          db,
		Tmpl:        tmpl,
		Mailer:      mailer,
		AsaasClient: asaasClient,
	}
}

// NewTemplateEngine cria uma nova instância do motor de templates
func NewTemplateEngine(baseDir string, devMode bool) *TemplateEngine {
	funcMap := template.FuncMap{
		"formatPrice": func(price float64) string {
			// Formata com 2 casas decimais e substitui ponto por vírgula para evitar erros de ponto flutuante
			str := fmt.Sprintf("%.2f", price)
			parts := strings.Split(str, ".")
			intPart := parts[0]
			decPart := parts[1]

			if len(intPart) > 3 {
				var result []string
				for i := len(intPart); i > 0; i -= 3 {
					start := i - 3
					if start < 0 {
						start = 0
					}
					result = append([]string{intPart[start:i]}, result...)
				}
				intPart = strings.Join(result, ".")
			}
			return fmt.Sprintf("R$ %s,%s", intPart, decPart)
		},
		"formatPriceRaw": func(price float64) string {
			return fmt.Sprintf("%.2f", price)
		},
		"safeHTML": func(s interface{}) template.HTML {
			switch v := s.(type) {
			case string:
				return template.HTML(v)
			case *string:
				if v != nil {
					return template.HTML(*v)
				}
				return template.HTML("{}")
			default:
				return template.HTML("{}")
			}
		},
		"getHour": func(hours map[string]map[string]string, day, field string) string {
			if hours == nil {
				return ""
			}
			if dayMap, ok := hours[day]; ok {
				return dayMap[field]
			}
			return ""
		},
		"hasDay": func(hours map[string]map[string]string, day string) bool {
			if hours == nil {
				return false
			}
			dayMap, ok := hours[day]
			if !ok {
				return false
			}
			return dayMap["open"] != "" && dayMap["close"] != ""
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"derefInt": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
		"isNil": func(p *int) bool {
			return p == nil
		},
		"formatTime": func(t time.Time) string {
			loc, err := time.LoadLocation("America/Sao_Paulo")
			if err == nil {
				t = t.In(loc)
			}
			return t.Format("02/01/2006 às 15:04")
		},
		"formatTimePtr": func(t *time.Time) string {
			if t == nil {
				return "Vitalício"
			}
			loc, err := time.LoadLocation("America/Sao_Paulo")
			var localTime time.Time
			if err == nil {
				localTime = t.In(loc)
			} else {
				localTime = *t
			}
			return localTime.Format("02/01/2006 às 15:04")
		},
		"isExpired": func(t *time.Time) bool {
			if t == nil {
				return false
			}
			return time.Now().After(*t)
		},
		"percent": func(current, max int) int {
			if max <= 0 {
				return 0
			}
			return int((float64(current) / float64(max)) * 100)
		},
		"multiply": func(price float64, qty int) float64 {
			return price * float64(qty)
		},
		"subtract": func(a, b float64) float64 {
			return a - b
		},
	}

	return &TemplateEngine{
		templates: make(map[string]*template.Template),
		baseDir:   baseDir,
		funcMap:   funcMap,
		devMode:   devMode,
	}
}

// Render renderiza um template com layout
func (te *TemplateEngine) Render(w http.ResponseWriter, layout, name string, data interface{}) error {
	if te.devMode {
		// Em desenvolvimento, sempre recarrega do disco para evitar cache
		tmpl, err := te.loadTemplate(layout, name)
		if err != nil {
			return fmt.Errorf("erro ao carregar template %s: %w", name, err)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return tmpl.ExecuteTemplate(w, "layout", data)
	}

	te.mu.RLock()
	tmpl, exists := te.templates[layout+":"+name]
	te.mu.RUnlock()

	if !exists {
		var err error
		tmpl, err = te.loadTemplate(layout, name)
		if err != nil {
			return fmt.Errorf("erro ao carregar template %s: %w", name, err)
		}

		te.mu.Lock()
		te.templates[layout+":"+name] = tmpl
		te.mu.Unlock()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "layout", data)
}

// RenderPartial renderiza um template parcial (sem layout, para HTMX)
func (te *TemplateEngine) RenderPartial(w http.ResponseWriter, name string, data interface{}) error {
	if te.devMode {
		// Em desenvolvimento, sempre recarrega do disco para evitar cache
		path := filepath.Join(te.baseDir, name)
		tmpl, err := template.New(filepath.Base(name)).Funcs(te.funcMap).ParseFiles(path)
		if err != nil {
			return fmt.Errorf("erro ao carregar partial %s: %w", name, err)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return tmpl.Execute(w, data)
	}

	te.mu.RLock()
	tmpl, exists := te.templates["partial:"+name]
	te.mu.RUnlock()

	if !exists {
		path := filepath.Join(te.baseDir, name)
		var err error
		tmpl, err = template.New(filepath.Base(name)).Funcs(te.funcMap).ParseFiles(path)
		if err != nil {
			return fmt.Errorf("erro ao carregar partial %s: %w", name, err)
		}

		te.mu.Lock()
		te.templates["partial:"+name] = tmpl
		te.mu.Unlock()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.Execute(w, data)
}

// RenderPage renderiza um template standalone (sem layout)
func (te *TemplateEngine) RenderPage(w http.ResponseWriter, name string, data interface{}) error {
	path := filepath.Join(te.baseDir, name)
	tmpl, err := template.New(filepath.Base(name)).Funcs(te.funcMap).ParseFiles(path)
	if err != nil {
		return fmt.Errorf("erro ao carregar página %s: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.Execute(w, data)
}

// loadTemplate carrega um template com seu layout
func (te *TemplateEngine) loadTemplate(layout, name string) (*template.Template, error) {
	layoutPath := filepath.Join(te.baseDir, "layouts", layout+".html")
	pagePath := filepath.Join(te.baseDir, name)

	tmpl, err := template.New(filepath.Base(layoutPath)).Funcs(te.funcMap).ParseFiles(layoutPath, pagePath)
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

// ReloadTemplates limpa o cache de templates (útil em desenvolvimento)
func (te *TemplateEngine) ReloadTemplates() {
	te.mu.Lock()
	te.templates = make(map[string]*template.Template)
	te.mu.Unlock()
}
