package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"keel.local/dev/internal/cmdb"
	"log"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: cmdb-schema versioned-extension.json")
	}
	ctx := context.Background()
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	service := cmdb.Service{DB: db}
	raw, e := os.ReadFile(os.Args[1])
	if e != nil {
		log.Fatal(e)
	}
	var patch cmdb.SchemaExtension
	if e = json.Unmarshal(raw, &patch); e != nil {
		log.Fatal(e)
	}
	base, e := service.Active(ctx)
	if e != nil {
		log.Fatal(e)
	}
	if base.Version == patch.Version {
		fmt.Println("Schema version already active:", base.Version)
		return
	}
	next, e := cmdb.Extend(base, patch)
	if e != nil {
		log.Fatal(e)
	}
	result, e := service.Install(ctx, next, base.Version, "controlled-schema-cli")
	if e != nil {
		log.Fatal(e)
	}
	fmt.Println(string(cmdb.Encode(result)))
}
