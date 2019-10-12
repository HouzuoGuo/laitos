package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
)

const HandleGitlabPage = `<html>
<head>
    <title>Gitlab browser</title>
</head>
<body>
    <form action="%s" method="post">
        <p>
            Shortcut name: <input type="password" name="shortcut" value="%s" />
            <br />
            Path: <input type="text" name="path" value="%s" />
            <input type="submit" name="submit" value="Go"/>
            <br />
            Download file from current path: <input type="text" name="file" value="%s" />
            <input type="submit" name="submit" value="Download"/>
        </p>
        <pre>%s</pre>
    </form>
</body>
</html>
` // Gitlab browser content

const (
	GitlabAPITimeoutSec = 110  // Timeout for outgoing API calls made to gitlab
	GitlabMaxObjects    = 4000 // GitlabMaxObjects is the maximum number of objects to list when browsing a git repository.
)

// Browse gitlab repositories and download repository files.
type HandleGitlabBrowser struct {
	PrivateToken string            `json:"PrivateToken"` // Gitlab user private token
	Projects     map[string]string `json:"Projects"`     // Project shortcut name VS "gitlab project ID"
	Recipients   []string          `json:"Recipients"`   // Recipients of notification emails
	MailClient   inet.MailClient   `json:"-"`            // MTA that delivers file download notification email

	logger lalog.Logger
}

func (lab *HandleGitlabBrowser) Initialise(logger lalog.Logger, _ *common.CommandProcessor) error {
	lab.logger = logger
	return nil
}

// An element of gitlab API "/repository/tree" response array.
type GitlabTreeObject struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	FullPath string `json:"path"`
}

/*
Call gitlab API to find out what directories and files are located under that path.
Directory names come with suffix forward-slash.
*/
func (lab *HandleGitlabBrowser) ListGitObjects(projectID string, paths string, maxEntries int) (dirs []string, fileNameID map[string]string, err error) {
	/*
		If there is a leading slash when browsing a directory, the result will definitely be empty. The user most likely
		wants to browse the directory's file list, therefore get rid of the leading slash.
		If present, a trailing slash does not matter.
	*/
	if len(paths) > 1 && paths[0] == '/' {
		paths = paths[1:]
	}
	dirs = make([]string, 0, 8)
	fileNameID = make(map[string]string)
	resp, err := inet.DoHTTP(inet.HTTPRequest{
		Header:     map[string][]string{"PRIVATE-TOKEN": {lab.PrivateToken}},
		TimeoutSec: GitlabAPITimeoutSec,
	}, "https://gitlab.com/api/v4/projects/%s/repository/tree?ref=master&recursive=false&per_page=%s&path=%s", projectID, maxEntries, paths)
	if err != nil {
		return
	} else if err = resp.Non2xxToError(); err != nil {
		return
	}
	var objects []GitlabTreeObject
	if err = json.Unmarshal(resp.Body, &objects); err != nil {
		return
	}
	for _, obj := range objects {
		if obj.Type == "blob" {
			fileNameID[obj.Name] = obj.ID
		} else {
			dirs = append(dirs, obj.Name+"/")
		}
	}
	sort.Strings(dirs)
	return
}

// Call gitlab API to download a file form git project.
func (lab *HandleGitlabBrowser) DownloadGitBlob(clientIP, projectID string, paths string, fileName string) (content []byte, err error) {
	// Download blob up to 256MB in size
	resp, err := inet.DoHTTP(inet.HTTPRequest{
		Header:     map[string][]string{"PRIVATE-TOKEN": {lab.PrivateToken}},
		TimeoutSec: GitlabAPITimeoutSec,
		MaxBytes:   256 * 1048576,
	}, "https://gitlab.com/api/v4/projects/%s/repository/files/%s/raw?ref=master", projectID, path.Join(paths, fileName))
	if err != nil {
		return
	} else if err = resp.Non2xxToError(); err != nil {
		return
	}
	content = resp.Body
	if lab.Recipients != nil && len(lab.Recipients) > 0 && lab.MailClient.IsConfigured() {
		go func() {
			subject := inet.OutgoingMailSubjectKeyword + "-gitlab-download-" + fileName
			if err := lab.MailClient.Send(subject, fmt.Sprintf("File \"%s/%s\" has been downloaded by %s", paths, fileName, clientIP), lab.Recipients...); err != nil {
				lab.logger.Warning("DownloadGitBlob", "", err, "failed to send notification for file \"%s\"", fileName)
			}
		}()
	}
	return
}

func (lab *HandleGitlabBrowser) Handle(w http.ResponseWriter, r *http.Request) {
	NoCache(w)
	shortcutName := strings.TrimSpace(r.FormValue("shortcut"))
	browsePath := r.FormValue("path")
	fileName := strings.TrimSpace(r.FormValue("file"))
	submitAction := r.FormValue("submit")
	switch submitAction {
	case "Go":
		w.Header().Set("Content-Type", "text/html")
		projectID, found := lab.Projects[shortcutName]
		if !found {
			_, _ = w.Write([]byte(fmt.Sprintf(HandleGitlabPage, r.RequestURI, shortcutName, browsePath, fileName, "(cannot find shortcut name)")))
			return
		}
		dirs, fileNames, err := lab.ListGitObjects(projectID, browsePath, GitlabMaxObjects)
		if err != nil {
			_, _ = w.Write([]byte(fmt.Sprintf(HandleGitlabPage, r.RequestURI, shortcutName, browsePath, fileName, "Error: "+err.Error())))
			return
		}
		// Directory names are already sorted
		contentList := strings.Join(dirs, "\n")
		// Sort file names
		sortedFiles := make([]string, 0, len(fileNames))
		for fileName := range fileNames {
			sortedFiles = append(sortedFiles, fileName)
		}
		sort.Strings(sortedFiles)
		contentList += "\n\n"
		for _, fileName := range sortedFiles {
			contentList += fmt.Sprintf("%s\n", fileName)
		}
		_, _ = w.Write([]byte(fmt.Sprintf(HandleGitlabPage, r.RequestURI, shortcutName, browsePath, fileName, contentList)))
	case "Download":
		projectID, found := lab.Projects[shortcutName]
		if !found {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(fmt.Sprintf(HandleGitlabPage, r.RequestURI, shortcutName, browsePath, fileName, "(cannot find shortcut name)")))
			return
		}
		content, err := lab.DownloadGitBlob(GetRealClientIP(r), projectID, browsePath, fileName)
		if err != nil {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(fmt.Sprintf(HandleGitlabPage, r.RequestURI, shortcutName, browsePath, fileName, "Error: "+err.Error())))
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		_, _ = w.Write(content)
	default:
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fmt.Sprintf(HandleGitlabPage, r.RequestURI, shortcutName, browsePath, fileName, "Enter path to browse or blob ID to download")))
	}
}

func (_ *HandleGitlabBrowser) GetRateLimitFactor() int {
	return 1
}

func (lab *HandleGitlabBrowser) SelfTest() error {
	errs := make([]error, 0)
	for shortcut, projectID := range lab.Projects {
		if _, _, err := lab.ListGitObjects(projectID, "/", 3); err != nil {
			errs = append(errs, fmt.Errorf("project %s(%s) - %v", shortcut, projectID, err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("HandleGitlabBrowser encountered errors: %+v", errs)
}
