package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Mensagem struct {
	ID        int
	Role      string // "user" ou "assistant"
	Content   string
	CreatedAt time.Time
}

type Database struct {
	db *sql.DB
}

// NovoDatabase cria e inicializa o banco SQLite
func NovoDatabase() (*Database, error) {
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	
	dbPath := filepath.Join(filepath.Dir(exePath), "Athena.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		dbPath = "Athena.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir banco: %w", err)
	}

	// Otimizações para performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=10000",
		"PRAGMA busy_timeout=5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("erro ao executar %s: %w", pragma, err)
		}
	}

	// Criar tabela se não existir
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS mensagens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar tabela: %w", err)
	}

	// Índice para busca por data
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_mensagens_data ON mensagens(created_at)
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar índice: %w", err)
	}

	fmt.Println("[Database] SQLite inicializado em:", dbPath)
	return &Database{db: db}, nil
}

// SalvarMensagem salva uma mensagem no banco
func (d *Database) SalvarMensagem(role, content string) error {
	_, err := d.db.Exec(
		"INSERT INTO mensagens (role, content, created_at) VALUES (?, ?, ?)",
		role, content, time.Now().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return fmt.Errorf("erro ao salvar mensagem: %w", err)
	}
	return nil
}

// BuscarMensagensDoDia retorna todas as mensagens de hoje
func (d *Database) BuscarMensagensDoDia() ([]Mensagem, error) {
	hoje := time.Now().Format("2006-01-02")

	rows, err := d.db.Query(
		"SELECT id, role, content, created_at FROM mensagens WHERE date(created_at) = ? ORDER BY created_at ASC",
		hoje,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar mensagens: %w", err)
	}
	defer rows.Close()

	var mensagens []Mensagem
	for rows.Next() {
		var m Mensagem
		var createdStr string
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &createdStr); err != nil {
			continue
		}
		m.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdStr)
		mensagens = append(mensagens, m)
	}

	return mensagens, nil
}

// Fechar fecha a conexão com o banco
func (d *Database) Fechar() {
	if d.db != nil {
		d.db.Close()
	}
}
