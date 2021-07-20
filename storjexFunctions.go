package main

import (
	"context"
	"fmt"

	"github.com/rs/xid"
)

func generatePassphrases(
    cxt context.Context,
    adminAccessToken string,
    userAccessToken string,
    bucket string,
    name string,
    objectKey string,
    numberOfDownloads int)(string, string) {

    randomPass1:=xid.New()
    randomPass2:=xid.New()
    adminPassphrase:="admin-" + name + "-" + randomPass1.String()
    userPassphrase:=name + "-" + randomPass2.String()
        // put the passes in database with access grant, bucket, key
    conn:=ConnectToDataBase()
    query:=fmt.Sprintf(`INSERT INTO passphrases (passphrase, adminPassphrase, adminAccessGrant, accessGrant, bucket, key, numberOfDownloads) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%v')`,
        userPassphrase, adminPassphrase, adminAccessToken, userAccessToken, bucket, objectKey, numberOfDownloads)
    if _,
    err:=conn.Exec(context.Background(),
        query);err != nil {
        fmt.Println(err)
    }
    return adminPassphrase,
    userPassphrase
}