package repository

import (
	"context"
	"database/sql"
)

type ServiceRegistryRepository interface {
	Register(ctx context.Context, serviceName, address string) error
	Deregister(ctx context.Context, serviceName string) error
	GetService(ctx context.Context, serviceName string) (string, error)
	ListServices(ctx context.Context) (map[string]string, error)
	UpdateLastSeen(ctx context.Context, serviceName string) error
}

type serviceRegistryRepository struct {
	db *sql.DB
}

func NewServiceRegistryRepository(db *sql.DB) ServiceRegistryRepository {
	return &serviceRegistryRepository{db: db}
}

func (r *serviceRegistryRepository) Register(ctx context.Context, serviceName, address string) error {
	query := `
		INSERT INTO service_registry (service_name, address, last_seen)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(service_name) 
		DO UPDATE SET 
			address = excluded.address, 
			last_seen = CURRENT_TIMESTAMP`

	_, err := r.db.ExecContext(ctx, query, serviceName, address)
	return err
}

func (r *serviceRegistryRepository) Deregister(ctx context.Context, serviceName string) error {
	query := `DELETE FROM service_registry WHERE service_name = ?`
	_, err := r.db.ExecContext(ctx, query, serviceName)
	return err
}

func (r *serviceRegistryRepository) GetService(ctx context.Context, serviceName string) (string, error) {
	var address string
	query := `SELECT address FROM service_registry WHERE service_name = ?`
	err := r.db.QueryRowContext(ctx, query, serviceName).Scan(&address)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return address, err
}

func (r *serviceRegistryRepository) ListServices(ctx context.Context) (map[string]string, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Prune stale services first
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM service_registry WHERE last_seen < datetime('now', '-30 seconds')`); err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, `SELECT service_name, address FROM service_registry`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	services := make(map[string]string)
	for rows.Next() {
		var name, addr string
		if err := rows.Scan(&name, &addr); err != nil {
			return nil, err
		}
		services[name] = addr
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return services, tx.Commit()
}

func (r *serviceRegistryRepository) UpdateLastSeen(ctx context.Context, serviceName string) error {
	query := `
		UPDATE service_registry 
		SET last_seen = CURRENT_TIMESTAMP 
		WHERE service_name = ?`

	_, err := r.db.ExecContext(ctx, query, serviceName)
	return err
}
