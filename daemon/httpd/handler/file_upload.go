package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/httpd/middleware"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/toolbox"
)

// HandleFileUploadPage is the HTML source code template of the file upload page.
const HandleFileUploadPage = `<html>
<head>
	<title>Upload temporary files for retrieval within 24 hours</title>
</head>
    <form action="%s" method="post" enctype="multipart/form-data">
        <p>
            <input type="submit" name="submit" value="Upload"/>
            <input type="file" name="upload" />
            <br /><br />
            <input type="submit" name="submit" value="Download"/>
            <input type="text" name="download" value="" />
        </p>
        <pre>%s</pre>
    </form>
</html>
`

const (
	// FileUploadMaxSizeBytes is the approximate maximum size of file acceptable for upload (~64MB).
	FileUploadMaxSizeBytes = 64 * 1024 * 1024
	// FileUploadCleanUpIntervalSec is the interval at which uploaded files are gone through one by one and outdated ones are deleted
	FileUploadCleanUpIntervalSec = 180
	// FileUploadExpireInSec is the expiration of uploaded files measured in seconds.
	FileUploadExpireInSec = 24 * 3600
)

// fileUploadStorage is the parent directory in which uploaded files are temporarily stored.
var fileUploadStorage = filepath.Join(os.TempDir(), "laitos-HandleFileUpload")

// fileUploadCleanUpStartOnce ensures that a background routine that removes expired files periodically is started exactly once.
var fileUploadCleanUpStartOnce = new(sync.Once)

// HandleFileUploadPage let visitors upload temporary files for retrieval within 24 hours
type HandleFileUpload struct {
	logger                     lalog.Logger
	stripURLPrefixFromResponse string
}

// Initialise prepares handler logger.
func (upload *HandleFileUpload) Initialise(logger lalog.Logger, _ *toolbox.CommandProcessor, stripURLPrefixFromResponse string) error {
	upload.logger = logger
	upload.stripURLPrefixFromResponse = stripURLPrefixFromResponse
	return nil
}

// render renders the file upload page in HTML
func (upload *HandleFileUpload) render(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	_, _ = w.Write([]byte(fmt.Sprintf(HandleFileUploadPage, strings.TrimPrefix(r.RequestURI, upload.stripURLPrefixFromResponse), message)))
}

// periodicallyDeleteExpiredFiles deletes expired files at regular interval. This function never returns.
func (upload *HandleFileUpload) periodicallyDeleteExpiredFiles() {
	for {
		time.Sleep(FileUploadCleanUpIntervalSec * time.Second)
		files, err := ioutil.ReadDir(fileUploadStorage)
		if err != nil {
			upload.logger.Warning("periodicallyDeleteExpiredFiles", "", err, "failed to read file upload directory")
			continue
		}
		var anyFileExpired bool
		for _, fileEntry := range files {
			// fileInfo, err := fileEntry.Info()
			if fileEntry.ModTime().Before(time.Now().Add(-(FileUploadExpireInSec * time.Second))) {
				anyFileExpired = true
				upload.logger.Info("periodicallyDeleteExpiredFiles", "", os.Remove(fileEntry.Name()), "delete expired file")
			}
		}
		if !anyFileExpired {
			upload.logger.Info("periodicallyDeleteExpiredFiles", "", nil, "did not find an expired file")
		}
	}
}

func (upload *HandleFileUpload) Handle(w http.ResponseWriter, r *http.Request) {
	fileUploadCleanUpStartOnce.Do(func() {
		go upload.periodicallyDeleteExpiredFiles()
	})
	NoCache(w)
	r.Body = http.MaxBytesReader(w, r.Body, FileUploadMaxSizeBytes)
	if r.Method != http.MethodGet {
		_ = r.ParseForm()
		_ = r.ParseMultipartForm(FileUploadMaxSizeBytes)
	}
	switch r.FormValue("submit") {
	case "Upload":
		uploadFile, fileHeader, err := r.FormFile("upload")
		if err != nil {
			http.Error(w, `failed to get input file`, http.StatusBadRequest)
			return
		}
		if fileHeader.Size > FileUploadMaxSizeBytes {
			http.Error(w, `input file size is too large`, http.StatusBadRequest)
			return
		}
		// Generate a random file name
		randName := make([]byte, 5)
		if _, err := rand.Read(randName); err != nil {
			http.Error(w, `failed to generate random file name`, http.StatusInternalServerError)
			return
		}
		// Generate a temporary file that preserves extension name of the original
		tmpFileName := hex.EncodeToString(randName) + filepath.Ext(fileHeader.Filename)
		if err := os.MkdirAll(fileUploadStorage, 0700); err != nil {
			http.Error(w, `failed to store file`, http.StatusInternalServerError)
			return
		}
		tmpFile, err := os.OpenFile(filepath.Join(fileUploadStorage, tmpFileName), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			http.Error(w, `failed to store file`, http.StatusInternalServerError)
			return
		}
		defer tmpFile.Close()
		// Copy the uploaded file
		if _, err := io.Copy(tmpFile, uploadFile); err != nil {
			http.Error(w, `failed to copy file content`, http.StatusInternalServerError)
			return
		}
		if err := tmpFile.Sync(); err != nil {
			http.Error(w, `failed to save file`, http.StatusInternalServerError)
			return
		}
		if err := tmpFile.Close(); err != nil {
			http.Error(w, `failed to close file`, http.StatusInternalServerError)
			return
		}
		upload.logger.Info("HandleFileUpload", middleware.GetRealClientIP(r), nil, "successfully saved file \"%s\" as \"%s\"", fileHeader.Filename, tmpFile.Name())
		upload.render(w, r, "Uploaded successfully. Your file is available for 24 hours under name: "+tmpFileName)
		return
	case "Download":
		downloadName := strings.TrimSpace(r.FormValue("download"))
		// Remember to prevent traversal attack
		if downloadName == "" || strings.ContainsAny(downloadName, `/\`) {
			upload.render(w, r, "Please enter a file name to download")
			return
		}
		stat, err := os.Stat(filepath.Join(fileUploadStorage, downloadName))
		if err != nil {
			upload.render(w, r, "File does not exist")
			return
		}
		if stat.Size() > FileUploadMaxSizeBytes {
			http.Error(w, `unexpected file size`, http.StatusInternalServerError)
			return
		}
		fh, err := os.Open(filepath.Join(fileUploadStorage, downloadName))
		if err != nil {
			http.Error(w, `failed to open file`, http.StatusInternalServerError)
			return
		}
		defer fh.Close()
		http.ServeFile(w, r, fh.Name())
	default:
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		_, _ = w.Write([]byte(fmt.Sprintf(HandleFileUploadPage, strings.TrimPrefix(r.RequestURI, upload.stripURLPrefixFromResponse), "")))
	}
}

func (_ *HandleFileUpload) GetRateLimitFactor() int {
	return 1
}

func (_ *HandleFileUpload) SelfTest() error {
	if err := os.MkdirAll(fileUploadStorage, 0700); err != nil {
		return fmt.Errorf("HandleFileUpload.SelfTest: failed to read/create storage directory \"%s\" - %v", fileUploadStorage, err)
	}
	return nil
}
