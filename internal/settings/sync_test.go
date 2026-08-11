package settings_test

import (
	"context"
	"path/filepath"
	"testing"

	"lrss/internal/db"
	"lrss/internal/settings"
)

func TestSyncConfig_RoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "s.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)

	cfg := settings.SyncConfig{
		Enabled:          true,
		Provider:         settings.SyncProviderS3,
		ObjectKey:        "mine.opml",
		S3Endpoint:       "http://127.0.0.1:9000",
		S3Region:         "us-east-1",
		S3Bucket:         "bucket",
		S3AccessKey:      "minio",
		S3SecretKey:      "minio123",
		S3ForcePathStyle: true,
		S3UseSSL:         false,
	}
	if err := store.SaveSyncConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadSyncConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != settings.SyncProviderS3 || got.S3Bucket != "bucket" || !got.Enabled {
		t.Fatalf("%+v", got)
	}
	if got.ObjectKey != "mine.opml" {
		t.Fatalf("objectKey = %q", got.ObjectKey)
	}
}

func TestSyncConfig_DefaultLoad(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "s2.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)
	got, err := store.LoadSyncConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled || got.Provider != settings.SyncProviderNone {
		t.Fatalf("%+v", got)
	}
}
