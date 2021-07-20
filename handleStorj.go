package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/rs/xid"
	"storj.io/uplink"
)

var apiKey = os.Getenv("STORJ_API_KEY")
var satelliteAddress = os.Getenv("SATELLITE_ADDRESS")

func UploadData(ctx context.Context,
    accessGrant, bucketName, name string,
    data[] byte, numberOfDownloads int, expiryDate time.Time)(string, string, string) {
    // TODO: Create two accesses - admin and user, and get passphrases for both.

    randomKey:=xid.New()
    objectKey:=randomKey.String()
        // Parse the admin Access Grant.
    project:=ConnectToStorjexProject(accessGrant)
        // Intitiate the upload of our Object to the specified bucket and key.
    upload,
    err:=project.UploadObject(ctx, bucketName, objectKey, nil)
    if err != nil {
        fmt.Errorf("could not initiate upload: %v", err)
    }

    // Copy the data to the upload.
    buf:=bytes.NewBuffer(data)
    _,
    err = io.Copy(upload, buf)
    if err != nil {
        _ = upload.Abort()
        fmt.Errorf("could not upload data: %v", err)
    }

    // Commit the uploaded object.
    err = upload.Commit()
    if err != nil {
        fmt.Errorf("could not commit uploaded object: %v", err)
    }

    // Create passphrases & user access token
    userAccessToken:=CreateAccessToken(name, objectKey, 0, expiryDate)
    adminAccessToken:=CreateAccessToken(name, objectKey, 1, expiryDate)

        adminPassphrase,
    userPassphrase:=generatePassphrases(
        context.Background(),
        adminAccessToken,
        userAccessToken,
        bucket,
        name,
        objectKey,
        numberOfDownloads)

    return userPassphrase,
    adminPassphrase,
    objectKey

}
func DownloadData(passphrase string)(fileContents[] byte, downloadsRemaining int, err error) {
    var accessGrant string
    var bucket string
    var key string
    var numberOfDownloads int
    ctx:=context.Background()

    conn:=ConnectToDataBase()

    query:=fmt.Sprintf(`SELECT accessGrant, bucket, key, numberOfDownloads FROM passphrases WHERE passphrase = '%s'`,
        passphrase)
    defer conn.Close(ctx)
    err = conn.QueryRow(ctx, query).Scan( & accessGrant, & bucket, & key, & numberOfDownloads)
    if err != nil {
        fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
    }
    if numberOfDownloads <= 0 {
        return nil, 0, fmt.Errorf("This file has reached its maximum number of downloads")
    } else {

        project:=ConnectToStorjexProject(accessGrant)
        download,
        err:=project.DownloadObject(ctx, bucket, key, nil)
        if err != nil {
            return nil, 0, fmt.Errorf("Could not open object: %v", err)
        }
        defer download.Close()
        remainingDownloads:=numberOfDownloads - 1

            query = fmt.Sprintf(`UPDATE passphrases SET numberOfDownloads = %v WHERE passphrase = '%s'`,
            remainingDownloads, passphrase)
        defer conn.Close(ctx)
        _,
        err = conn.Query(ctx, query)
        if err != nil {
            return nil, 0, fmt.Errorf("Could not get file from database: %v", err)
        }

        // Read everything from the download stream
        receivedContents,
        err:=ioutil.ReadAll(download)
        if err != nil {
            return nil, 0, fmt.Errorf("Could not read file data, may be corrupted: %v", err)
        }

        // Check that the downloaded data is the same as the uploaded data.
        return receivedContents,
        remainingDownloads,
        nil
    }
}

func CreateAccessToken(name string, objectKey string, level int, expiryDate time.Time) string {
    // TODO: create two access tokens - one admin (can
    // delete and update object) and one just for downloading. Call twice
    now:=time.Now()

    var permission uplink.Permission
    if level == 0 {
        permission = uplink.FullPermission()
    } else {
        permission = uplink.ReadOnlyPermission()
    }
    // set variables for user permission
    // TODO: THIS DOES NOT WORK
    access,
    err:=uplink.RequestAccessWithPassphrase(context.Background(),
        satelliteAddress, apiKey, "elephant")
    if err != nil {
        fmt.Println(err)
    }
    duration:=expiryDate.Sub(now)

    // create an access grant for reading bucket "storjex"
        shared:=uplink.SharePrefix {
        Bucket: bucket,
        Prefix: objectKey,
    }
    permission.NotBefore = now.Add(-2 * time.Minute)
    permission.NotAfter = now.Add(duration)
    restrictedAccess,
    err:=access.Share(permission, shared)
        // serialize the restricted access grant
    serializedAccess,
    err:=restrictedAccess.Serialize()

    return serializedAccess
}

func HandleDelete(passphrase string) string {
    var bucket string
    var key string
    var accessGrant string
    ctx:=context.Background()
    conn:=ConnectToDataBase()

    query:=fmt.Sprintf(`SELECT adminAccessGrant, bucket, key FROM passphrases WHERE adminPassphrase = '%s'`,
        passphrase)

    err:=conn.QueryRow(ctx, query).Scan( & accessGrant, & bucket, & key)
    if err != nil {
        return fmt.Sprintf("Could not find file: %v\n", err)
    }

    project:=ConnectToStorjexProject(myAccessGrant)

    _, err = project.DeleteObject(ctx, bucket, key)
    if err != nil {
        return fmt.Sprintf("Could not delete file: %v", err)
    }

    query = fmt.Sprintf(`DELETE FROM passphrases WHERE adminPassphrase = '%s'`,
        passphrase)

    _, err = conn.Query(ctx, query)
    if err != nil {
        return fmt.Sprintf("Could not delete from database: %v\n", err)
    }

    return "Successfully deleted file"

}

func HandleUpdateFile(passphrase string, newNumberOfDownloads int) string {
    ctx:=context.Background()
    conn:=ConnectToDataBase()

        query:=fmt.Sprintf(`UPDATE passphrases SET numberOfDownloads = %v WHERE adminPassphrase = '%s'`,
        newNumberOfDownloads, passphrase)
    defer conn.Close(ctx)
    _,
    err:=conn.Query(ctx, query)
    if err != nil {
        return fmt.Sprintf("Could not get file from database: %v", err)
    }
    return fmt.Sprintf("Successfully updated number of downloads to %v", newNumberOfDownloads)
}