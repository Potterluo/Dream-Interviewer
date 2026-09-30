package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// files.go implements the file-metadata slice of Store. File BYTES live on
// disk under <DataDir>/files/<id>; only metadata is in the database — the
// standard split for local-disk object storage (swap in S3/MinIO later by
// keeping this table and replacing the read/write paths in handlers_files.go).

type FileRecord struct {
	ID          string
	UserID      string
	Name        string // original filename as uploaded
	Size        int64
	ContentType string
	CreatedAt   time.Time
}

const fileColumns = `id, user_id, name, size, content_type, created_at`

func scanFileRecord(scanner interface{ Scan(dest ...any) error }) (*FileRecord, error) {
	var f FileRecord
	if err := scanner.Scan(&f.ID, &f.UserID, &f.Name, &f.Size, &f.ContentType, &f.CreatedAt); err != nil {
		return nil, err
	}
	return &f, nil
}

func (d *DBStore) CreateFileRecord(ctx context.Context, f *FileRecord) error {
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}
	_, err := d.db.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO files (id, user_id, name, size, content_type, created_at) VALUES (%s, %s, %s, %s, %s, %s)`,
			d.ph(1), d.ph(2), d.ph(3), d.ph(4), d.ph(5), d.ph(6)),
		f.ID, f.UserID, f.Name, f.Size, f.ContentType, f.CreatedAt)
	return err
}

func (d *DBStore) GetFileRecord(ctx context.Context, id string) (*FileRecord, error) {
	row := d.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT `+fileColumns+` FROM files WHERE id = %s`, d.ph(1)), id)
	f, err := scanFileRecord(row)
	return f, scanErr(err)
}

func (d *DBStore) ListFileRecords(ctx context.Context, userID string) ([]FileRecord, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if userID == "" {
		rows, err = d.db.QueryContext(ctx,
			fmt.Sprintf(`SELECT %s FROM files ORDER BY created_at DESC`, fileColumns))
	} else {
		rows, err = d.db.QueryContext(ctx,
			fmt.Sprintf(`SELECT %s FROM files WHERE user_id = %s ORDER BY created_at DESC`, fileColumns, d.ph(1)),
			userID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FileRecord
	for rows.Next() {
		f, err := scanFileRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

func (d *DBStore) DeleteFileRecord(ctx context.Context, id string) error {
	res, err := d.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM files WHERE id = %s`, d.ph(1)), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
