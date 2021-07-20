package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v4"
	"storj.io/uplink"
)

func ConnectToDataBase() * pgx.Conn {
    ctx:=context.Background()
    config,
    err:=pgx.ParseConfig(os.ExpandEnv("postgresql://cat:635mAVjxfz9woBC9@free-tier7.aws-eu-west-1.cockroachlabs.cloud:26257/storjex?sslmode=verify-full&sslrootcert=$HOME/.postgresql/root.crt&options=--cluster%3Dwarped-yak-139"))
    if err != nil {
        log.Fatal("error configuring the database: ", err)
    }
    
    conn, err:=pgx.ConnectConfig(context.Background(), config) // Connect to the "storjex" database
    if err != nil {
        log.Fatal("error connecting to the database: ", err)
    }

    if _, err:=conn.Exec(ctx,
        "CREATE TABLE IF NOT EXISTS passphrases (passphrase STRING, adminPassphrase STRING, adminAccessGrant STRING, accessGrant STRING, bucket STRING, key STRING, numberOfDownloads INT)");err != nil {
        log.Fatal(err) 
    } // Create the "passphrases" table
 
    return conn
}

func ConnectToStorjexProject(accessGrant string) * uplink.Project {
    ctx:=context.Background()
    access,
    err:=uplink.ParseAccess(accessGrant) // Parse serialized access grant
    if err != nil {
        fmt.Errorf("could not parse access grant: %v", err)
    }
    
    project, err:=uplink.OpenProject(ctx, access) // Open project to be ready for upload/download/update of object
    if err != nil { 
        fmt.Errorf("could not open project: %v", err)
    }
    defer project.Close()

    _,
    err = project.EnsureBucket(ctx, bucket) // Make sure that Storjex bucket exists
    if err != nil {
        fmt.Errorf("could not ensure bucket: %v", err)
    }
    return project
}