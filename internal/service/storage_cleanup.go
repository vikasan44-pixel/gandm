package service

import (
	"context"
	"fmt"
	"time"

	"gandm/internal/storage"
)

// cleanupUploadedObject compensates an S3 upload when the following database
// write fails. It deliberately ignores cancellation of the request that caused
// the failure and uses a short independent timeout for the best-effort cleanup.
func cleanupUploadedObject(ctx context.Context, client *storage.S3Client, key string, cause error) error {
	if err := deleteStoredObject(ctx, client, key); err != nil {
		return fmt.Errorf("%w; uploaded object cleanup failed: %v", cause, err)
	}
	return cause
}

func deleteStoredObject(ctx context.Context, client *storage.S3Client, key string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return client.Delete(cleanupCtx, key)
}
