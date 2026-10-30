package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/minio/minio-go/v7"
	"keel.local/dev/internal/bom"
	"net/http"
	"strings"
	"time"
)

func (s *Server) packageAPI(w http.ResponseWriter, r *http.Request, id string, rev int, draft []byte) bool {
	download := strings.HasSuffix(r.URL.Path, "/package")
	build := strings.HasSuffix(r.URL.Path, "/build-package")
	if !download && !build {
		return false
	}
	if (download && r.Method != "GET") || (build && r.Method != "POST") {
		return false
	}
	var items []bom.Item
	json.Unmarshal(draft, &items)
	var committed *string
	s.db.QueryRow(r.Context(), "SELECT committed_revision::text FROM bom_import_batch WHERE id=$1", id).Scan(&committed)
	if committed != nil {
		rows, e := s.db.Query(r.Context(), "SELECT payload FROM bom_item_revision WHERE revision_id::text=$1 ORDER BY item_uid", *committed)
		if e != nil {
			fail(w, 500, "DATABASE_ERROR")
			return true
		}
		items = []bom.Item{}
		for rows.Next() {
			var raw []byte
			if rows.Scan(&raw) != nil {
				rows.Close()
				fail(w, 500, "DATABASE_ERROR")
				return true
			}
			var item bom.Item
			if json.Unmarshal(raw, &item) != nil {
				rows.Close()
				fail(w, 500, "SNAPSHOT_INVALID")
				return true
			}
			items = append(items, item)
		}
		rows.Close()
	} else {
		if hasBlockers(s.validateDraft(r.Context(), id, items)) {
			fail(w, 422, "BLOCKING_ISSUES_PRESENT")
			return true
		}
		items, e := bom.Normalize(items)
		if e != nil {
			fail(w, 422, "REFERENCE_UNRESOLVED")
			return true
		}
		for j := range items {
			item := &items[j]
			if item.Model != "" {
				var ref string
				if s.db.QueryRow(r.Context(), "SELECT id::text FROM device_model_revision WHERE model=$1 AND category=$2 AND ($3='' OR id::text=$3)", item.Model, item.Category, item.Fields["model_version_ref"]).Scan(&ref) != nil {
					fail(w, 422, "MODEL_REVISION_REQUIRED")
					return true
				}
				item.Fields["model_version_ref"] = ref
			}
		}
		for j := range items {
			items[j].Status = "planned"
		}
	}

	data, e := bom.Package(items, id, s.project)
	if e != nil {
		fail(w, 500, "PACKAGE_BUILD_FAILED")
		return true
	}
	h := sha256.Sum256(data)
	hash := hex.EncodeToString(h[:])
	key := fmt.Sprintf("%s/packages/%s/%d/%s.zip", s.project, id, rev, hash)
	_, e = s.objects.PutObject(r.Context(), s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/zip"})
	if e != nil {
		fail(w, 503, "OBJECT_STORAGE_UNAVAILABLE")
		return true
	}
	s.audit(context.WithoutCancel(r.Context()), id, "package.built", hash)
	if download {
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="keel-bom.zip"`)
		w.Write(data)
		return true
	}
	respond(w, 202, map[string]any{"created_at": time.Now().UTC(), "id": id, "kind": "build-package", "status": "succeeded", "resource_id": id, "artifact_sha256": hash, "download_url": "/api/v1/bom-imports/" + id + "/package"})
	return true
}
