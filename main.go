package main

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type server struct {}

var bucket = os.Getenv("STORJ_BUCKET")
var myAccessGrant = os.Getenv("ACCESS_GRANT")

func main() {
    r:=mux.NewRouter()
    api:=r.PathPrefix("/api/v1").Subrouter()
    api.HandleFunc("", uploadFile).Methods(http.MethodPost)
    api.HandleFunc("/file/{passphrase}", deleteFile).Methods("DELETE", "OPTIONS")
    api.HandleFunc("/file/{passphrase}", downloadFile).Methods(http.MethodGet)
    api.HandleFunc("/file/{passphrase}", updateFile).Methods("PUT")
    log.Fatal(http.ListenAndServe(":" + os.Getenv("BACKEND_PORT"), r))
}

func uploadFile(w http.ResponseWriter, r * http.Request) {
    r.ParseMultipartForm(10 << 20) // Parse form data
    file, _, err:=r.FormFile("file")
    if err != nil {
        message:=fmt.Sprint("Error parsing form: ", err)
        w.Write([] byte(message))
    }
    name:=r.FormValue("name")
    var numberOfDownloads int
    if r.FormValue("numberOfDownloads") == "" { // Set default number of downloads if none entered
        numberOfDownloads = 10000
    } else {
        numberOfDownloads, err = strconv.Atoi(r.FormValue("numberOfDownloads")) // Form value is string, convert into int
        if err != nil {
            message:=fmt.Sprint("Error parsing form: ", err)
            w.Write([] byte(message))
        }
    }
    var expiryDate string

    layout:="2006-01-02T15:04:05.000Z" // Use this layout to convert string into date
    var dateTime time.Time
    if r.FormValue("expiryDate") == "" { // Default expiry = a year from now
        now:=time.Now()
        dateTime = now.Add(8766 * time.Hour)

    } else {

        expiryDate = fmt.Sprintf("%sT23:59:59.000Z", r.FormValue("expiryDate"))
        dateTime, err = time.Parse(layout, expiryDate)
        if err != nil {
            fmt.Println(err)
        }
    }

    defer file.Close() 
    var fileByte[] byte 
    fileByte, byteErr:=ioutil.ReadAll(file) // Convert file into bytes
    if byteErr != nil {
        message:=fmt.Sprint("Error converting file: ", err)
        w.Write([] byte(message))
    }

    passphrase, adminPassphrase, objectKey:=UploadData(context.Background(),
        myAccessGrant, bucket, name, fileByte, numberOfDownloads, dateTime) // Upload data to Storj & Storjex DB

    w.WriteHeader(http.StatusOK)
    w.Header().Set("Content-Type", "application/json")
    message:=fmt.Sprintf(`{"passphrase": "%s", "adminPassphrase": "%s", "bucket": "%s", "key": "%s"}`, passphrase, adminPassphrase, bucket, objectKey)
    w.Write([] byte(message)) // Return file details including passphrases to download, update, and delete
}

func downloadFile(w http.ResponseWriter, r * http.Request) {
    pathParams:=mux.Vars(r) // Get passphrase as path parameter (file/v1/passphrase)
    passphrase:=pathParams["passphrase"]
    fileByte,
    downloadsRemaining,
    err:=DownloadData(passphrase)
    if err != nil {
        message:=fmt.Sprint("Error: ", err)
        w.Write([] byte(message))
    } else {
        encodedString:=b64.StdEncoding.EncodeToString([] byte(fileByte))
        message:=fmt.Sprintf(`{"image": "%s", "downloadsRemaining": "%v"}`, encodedString, downloadsRemaining)
        w.Header().Set("Content-Type", "application/json")
        w.Write([] byte(message)) // Return downloads remaining as JSON
    }
}

func deleteFile(w http.ResponseWriter, r * http.Request) {
    header:=w.Header() // Fix CORS issues
    header.Add("Access-Control-Allow-Origin", "*")
    header.Add("Access-Control-Allow-Methods", "DELETE, POST, GET, OPTIONS")
    header.Add("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")

    if r.Method == "OPTIONS" {
        w.WriteHeader(http.StatusOK)
        return
    } else {
        pathParams:=mux.Vars(r)
        passphrase:=pathParams["passphrase"] // Get passphrase from path param, i.e. api/v1/passphrase
        phrase:=HandleDelete(passphrase)
        response:=fmt.Sprintf(`{"response": "%s"}`, phrase)
        w.Header().Set("Content-Type", "application/json")
        w.Write([] byte(response))
    }
}

func updateFile(w http.ResponseWriter, r * http.Request) {
    pathParams:=mux.Vars(r) // Get passphrase from path param, i.e. api/v1/passphrase
    passphrase:=pathParams["passphrase"]
    r.ParseMultipartForm(10 << 20) // Get number of downloads from form data
    newNumberOfDownloads,
    err:=strconv.Atoi(r.FormValue("numberOfDownloads"))
    if err != nil {
        fmt.Println(err)
    }
    w.Header().Set("Content-Type", "application/json")
    response:=HandleUpdateFile(passphrase, newNumberOfDownloads)
    res:=fmt.Sprintf(`{"response": "%s"}`, response)
    w.Write([] byte(res))
}