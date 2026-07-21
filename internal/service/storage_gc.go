package service

import (
	"context"
	"fmt"
	"time"

	"gandm/internal/repository"
	"gandm/internal/storage"
)

var storedObjectPrefixes = []string{"documents/", "vehicle-documents/", "fill-reports/", "deal-docs/"}

// CollectOrphanedStorageObjects removes old objects that are not referenced by
// any document table. The age guard prevents racing a fresh upload whose DB
// transaction has not committed yet.
func CollectOrphanedStorageObjects(ctx context.Context, q repository.Querier, client *storage.S3Client, minimumAge time.Duration) (int, error) {
	rows, err := q.Query(ctx, `
		SELECT file_url FROM documents
		UNION SELECT file_url FROM vehicle_documents
		UNION SELECT photo_url FROM warehouse_fill_reports WHERE photo_url IS NOT NULL
		UNION SELECT file_url FROM deal_documents`)
	if err != nil {
		return 0, err
	}
	referenced := make(map[string]struct{})
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return 0, err
		}
		referenced[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	cutoff := time.Now().Add(-minimumAge)
	removed := 0
	for _, prefix := range storedObjectPrefixes {
		objects, err := client.List(ctx, prefix)
		if err != nil {
			return removed, fmt.Errorf("list %s: %w", prefix, err)
		}
		for _, object := range objects {
			if _, ok := referenced[object.Key]; ok || object.LastModified.After(cutoff) {
				continue
			}
			if err := client.Delete(ctx, object.Key); err != nil {
				return removed, fmt.Errorf("delete %s: %w", object.Key, err)
			}
			removed++
		}
	}
	return removed, nil
}
