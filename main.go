// TO DO
// Get access grants working
// Deploy
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

	"github.com/gorilla/mux"
)

type server struct{}

var bucket = os.Getenv("STORJ_BUCKET")
var myAccessGrant = os.Getenv("ACCESS_GRANT")


func main() {
    r := mux.NewRouter()
    api := r.PathPrefix("/api/v1").Subrouter()
    api.HandleFunc("", uploadFile).Methods(http.MethodPost)
    api.HandleFunc("", deleteFile).Methods(http.MethodDelete)
    api.HandleFunc("/file/{passphrase}", downloadFile).Methods(http.MethodGet)
	// api.HandleFunc("/fileInfo", getFileInfo).Methods(http.MethodGet)
    log.Fatal(http.ListenAndServe(":" + os.Getenv("PORT"), r))

}

func uploadFile(w http.ResponseWriter, r * http.Request) {
    r.ParseMultipartForm(10 << 20)
	file, _, err:= r.FormFile("file")
    if err != nil {
        message := fmt.Sprint("Error parsing form: ", err)
        w.Write([]byte(message)); 
    } 
	name := r.FormValue("name")
    var numberOfDownloads int
    if (r.FormValue("numberOfDownloads") == "") {
        numberOfDownloads = 10000
    } else {
        numberOfDownloads, err = strconv.Atoi(r.FormValue("numberOfDownloads"))
            if err != nil {
          message := fmt.Sprint("Error parsing form: ", err)
             w.Write([]byte(message)); 
     }
    }
    defer file.Close()
    var fileByte[] byte
      fileByte, byteErr:= ioutil.ReadAll(file)
    if byteErr != nil {
        message := fmt.Sprint("Error converting file: ", err)
        w.Write([]byte(message)); 
    }

    passphrase, adminPassphrase, objectKey := UploadData(context.Background(),
        myAccessGrant, bucket, name, fileByte, numberOfDownloads)

	w.WriteHeader(http.StatusOK)
	message := fmt.Sprintf(`{"passphrase": "%s", "adminPassphrase": "%s", "bucket": "%s", "key": "%s"}`, passphrase, adminPassphrase, bucket, objectKey)
    w.Write([]byte(message))
}


func downloadFile(w http.ResponseWriter, r * http.Request) {
	pathParams := mux.Vars(r)
    passphrase := pathParams["passphrase"]
	        // pass this passphrase to download data
    fileByte, downloadsRemaining, err := DownloadData(passphrase)
    if err != nil {
        message := fmt.Sprint("Error: ", err)
        w.Write([]byte(message)); 
    } else {
	encodedString := b64.StdEncoding.EncodeToString([]byte(fileByte))
	message := fmt.Sprintf(`{"image": "%s", "downloadsRemaining: "%v"}`, encodedString, downloadsRemaining)
	w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(message)); 
}}

func deleteFile(w http.ResponseWriter, r * http.Request) {
	r.ParseMultipartForm(10 << 20)
	passphrase := r.FormValue("adminPassphrase")
	response := HandleDelete(passphrase) 
    w.Write([]byte(response))
}


