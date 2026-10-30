package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"keel.local/dev/internal/cmdb"
	"log"
	"os"
)

func main() {
	if os.Getenv("APP_ENV") == "production" {
		log.Fatal("synthetic fixture import is disabled in production")
	}
	if len(os.Args) != 2 {
		log.Fatal("usage: cmdb-seed canonical-demo-seed.json")
	}
	ctx := context.Background()
	raw, e := os.ReadFile(os.Args[1])
	if e != nil {
		log.Fatal(e)
	}
	var data struct {
		Collections cmdb.SeedCollections `json:"collections"`
	}
	if e = json.Unmarshal(raw, &data); e != nil {
		log.Fatal(e)
	}
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	service := cmdb.Service{DB: db}
	if e = service.Initialize(ctx); e != nil {
		log.Fatal(e)
	}
	objects, e := minio.New(os.Getenv("S3_ENDPOINT"), &minio.Options{Creds: credentials.NewStaticV4(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), ""), Secure: false})
	if e != nil {
		log.Fatal(e)
	}
	keys := map[string]string{}
	for _, resource := range cmdb.EvidenceResources {
		groups := map[string][]map[string]any{}
		for _, r := range data.Collections[resource] {
			project, _ := r["project_id"].(string)
			if project == "" {
				project = "proj-b300-demo"
			}
			groups[project] = append(groups[project], r)
		}
		for project, rows := range groups {
			key := "cmdb/synthetic/" + hash + "/" + project + "/" + resource + ".json"
			payload := cmdb.Encode(rows)
			if _, e = objects.PutObject(ctx, os.Getenv("S3_BUCKET"), key, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{ContentType: "application/json"}); e != nil {
				log.Fatal(e)
			}
			keys[resource+":"+project] = key
		}
	}
	result, e := service.Seed(ctx, data.Collections, hash, keys)
	if e != nil {
		log.Fatal(e)
	}
	fmt.Println(string(cmdb.Encode(result)))
	refreshed, e := service.RefreshFixtureReferences(ctx, data.Collections)
	if e != nil {
		log.Fatal(e)
	}
	fmt.Println(string(cmdb.Encode(refreshed)))
}
