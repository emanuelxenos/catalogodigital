package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB encapsula o pool de conexões PostgreSQL
type DB struct {
	Pool *pgxpool.Pool
}

// Connect cria um pool de conexões com o PostgreSQL
func Connect(databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear DATABASE_URL: %w", err)
	}

	// Configurações do pool
	config.MaxConns = 25
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao banco: %w", err)
	}

	// Testa a conexão
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("erro ao pingar o banco: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// RunMigrations executa o schema.sql para criar as tabelas
func (db *DB) RunMigrations() error {
	// Encontra o diretório raiz do projeto
	sqlPath := findSQLPath()

	schema, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("erro ao ler schema.sql: %w", err)
	}

	_, err = db.Pool.Exec(context.Background(), string(schema))
	if err != nil {
		return fmt.Errorf("erro ao executar migrações: %w", err)
	}

	return nil
}

// Close fecha o pool de conexões
func (db *DB) Close() {
	db.Pool.Close()
}

// findSQLPath tenta localizar o arquivo schema.sql
func findSQLPath() string {
	// Tenta caminhos relativos comuns
	paths := []string{
		"sql/schema.sql",
		"../sql/schema.sql",
		"../../sql/schema.sql",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fallback: usa o caminho relativo ao arquivo fonte
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "..", "..", "sql", "schema.sql")
}
