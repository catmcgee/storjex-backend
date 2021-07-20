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

    randomKey:=xid.New()
    objectKey:=randomKey.String()
    project:=ConnectToStorjexProject(accessGrant) // Connect to Storjex project with main access grant
    upload, err:=project.UploadObject(ctx, bucketName, objectKey, nil) // Initiate upload
    if err != nil {
        fmt.Errorf("could not initiate upload: %v", err)
    }

    buf:=bytes.NewBuffer(data) // Copy file to upload
    _, err = io.Copy(upload, buf)
    if err != nil {
        _ = upload.Abort()
        fmt.Errorf("could not upload data: %v", err)
    }

    err = upload.Commit() // Commit the uploaded object to Storj
    if err != nil {
        fmt.Errorf("could not commit uploaded object: %v", err)
    }

    userAccessToken:=CreateAccessToken(name, objectKey, 0, expiryDate)
    adminAccessToken:=CreateAccessToken(name, objectKey, 1, expiryDate)

    adminPassphrase, userPassphrase:=generatePassphrases(
        context.Background(),
        adminAccessToken,
        userAccessToken,
        bucket,
        name,
        objectKey,
        numberOfDownloads)

    return userPassphrase, adminPassphrase, objectKey
}

func DownloadData(passphrase string)(fileContents[] byte, downloadsRemaining int, err error) {
    var accessGrant string
    var bucket string
    var key string
    var numberOfDownloads int
    ctx:=context.Background()

    conn:=ConnectToDataBase()

    query:=fmt.Sprintf(`SELECT accessGrant, bucket, key, numberOfDownloads FROM passphrases WHERE passphrase = '%s'`,
        passphrase) // Find access grant & object information from the passphrase entered
    defer conn.Close(ctx)
    err = conn.QueryRow(ctx, query).Scan( & accessGrant, & bucket, & key, & numberOfDownloads)
    if err != nil {
        fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
    }
    if numberOfDownloads <= 0 { // Ensure there are enough downloads left - this is stored in DB
        return nil, 0, fmt.Errorf("This file has reached its maximum number of downloads")
    
     } else {
        project:=ConnectToStorjexProject(accessGrant)
        download, err:=project.DownloadObject(ctx, bucket, key, nil) // Initiate download
        if err != nil {
            return nil, 0, fmt.Errorf("Could not open object: %v", err)
        }
        defer download.Close()
        remainingDownloads:=numberOfDownloads - 1 // Update number of downloads in DB

        query = fmt.Sprintf(`UPDATE passphrases SET numberOfDownloads = %v WHERE passphrase = '%s'`,
        remainingDownloads, passphrase)
        defer conn.Close(ctx)
        _, err = conn.Query(ctx, query)
        if err != nil {
            return nil, 0, fmt.Errorf("Could not get file from database: %v", err)
        }

        
        receivedContents, err:=ioutil.ReadAll(download) // Read everything from the download stream
        if err != nil {
            return nil, 0, fmt.Errorf("Could not read file data, may be corrupted: %v", err)
        }

        return receivedContents, remainingDownloads, nil
    }
}

func CreateAccessToken(name string, objectKey string, level int, expiryDate time.Time) string {
   
    now:=time.Now() // 

    var permission uplink.Permission
    if level == 0 { // Create admin permission (can delete & update specified object)
        permission = uplink.FullPermission()
    } else {
        permission = uplink.ReadOnlyPermission() // Create permission that can only download file
    }

    access, err:=uplink.RequestAccessWithPassphrase(context.Background(),
        satelliteAddress, apiKey, os.Getenv("STORJ_PASSPHRASE"))
    if err != nil {
        fmt.Println(err)   
    }

    duration:=expiryDate.Sub(now) // Calculate duration from now until expiry date entered
        shared:=uplink.SharePrefix { // Can only access this object in bucket
        Bucket: bucket,
        Prefix: objectKey,
    }
    permission.NotBefore = now.Add(-2 * time.Minute) // In case of satellite issues
    permission.NotAfter = now.Add(duration) // Can only access the file from now until expiry date
    restrictedAccess, err:=access.Share(permission, shared)
    serializedAccess, err:=restrictedAccess.Serialize() // Serialize access grant

    return serializedAccess
}

func HandleDelete(passphrase string) string {
    var bucket string
    var key string
    var accessGrant string
    ctx:=context.Background()
    conn:=ConnectToDataBase()

    query:=fmt.Sprintf(`SELECT adminAccessGrant, bucket, key FROM passphrases WHERE adminPassphrase = '%s'`,
        passphrase) // Find information about object from admin passphrase entered

    err:=conn.QueryRow(ctx, query).Scan( & accessGrant, & bucket, & key)
    if err != nil {
        return fmt.Sprintf("Could not find file: %v\n", err)
    }

    project:=ConnectToStorjexProject(accessGrant)

    _, err = project.DeleteObject(ctx, bucket, key) // Delete object from Storj
    if err != nil {
        return fmt.Sprintf("Could not delete file: %v", err)
    }

    query = fmt.Sprintf(`DELETE FROM passphrases WHERE adminPassphrase = '%s'`,
        passphrase) // Delete object from Storjex

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
        newNumberOfDownloads, passphrase) // Update number of downloads from the file with admin passphrase entered
    defer conn.Close(ctx)
    _,
    err:=conn.Query(ctx, query)
    if err != nil {
        return fmt.Sprintf("Could not get file from database: %v", err)
    }
    return fmt.Sprintf("Successfully updated number of downloads to %v", newNumberOfDownloads)
}