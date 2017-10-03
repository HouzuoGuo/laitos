package api

import (
	"encoding/json"
	"fmt"
	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"net/http"
	"sort"
	"strings"
)

const HandleGitlabPage = `<!doctype html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
    <title>Gitlab browser</title>
</head>
<body>
    <form action="#" method="get">
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

const GitlabAPITimeoutSec = 110 // Timeout for outgoing API calls made to gitlab

// Browse gitlab repositories and download repository files.
type HandleGitlabBrowser struct {
	PrivateToken string            `json:"PrivateToken"` // Gitlab user private token
	Projects     map[string]string `json:"Projects"`     // Project shortcut name VS "gitlab project ID"
	Recipients   []string          `json:"Recipients"`   // Recipients of notification emails
	Mailer       inet.Mailer       `json:"-"`            // MTA that delivers file download notification email
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
func (lab *HandleGitlabBrowser) ListGitObjects(projectID string, paths string) (dirs []string, fileNameID map[string]string, err error) {
	dirs = make([]string, 0, 8)
	fileNameID = make(map[string]string)
	resp, err := inet.DoHTTP(inet.Request{
		Header:     map[string][]string{"PRIVATE-TOKEN": {lab.PrivateToken}},
		TimeoutSec: GitlabAPITimeoutSec,
	}, "https://gitlab.com/api/v4/projects/%s/repository/tree?path=%s", projectID, paths)
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
func (lab *HandleGitlabBrowser) DownloadGitBlob(logger misc.Logger, clientIP, projectID string, paths string, fileName string) (content []byte, err error) {
	// Call tree API to determine object ID
	_, fileIDName, err := lab.ListGitObjects(projectID, paths)
	if err != nil {
		return
	}
	objectID := fileIDName[fileName]
	// Call another API to download blob content
	resp, err := inet.DoHTTP(inet.Request{
		Header:     map[string][]string{"PRIVATE-TOKEN": {lab.PrivateToken}},
		TimeoutSec: GitlabAPITimeoutSec,
	}, "https://gitlab.com/api/v4/projects/%s/repository/blobs/%s/raw", projectID, objectID)
	if err != nil {
		return
	} else if err = resp.Non2xxToError(); err != nil {
		return
	}
	content = resp.Body
	if lab.Recipients != nil && len(lab.Recipients) > 0 && lab.Mailer.IsConfigured() {
		go func() {
			subject := inet.OutgoingMailSubjectKeyword + "-gitlab-download-" + fileName
			if err := lab.Mailer.Send(subject, fmt.Sprintf("File \"%s\" has been downloaded by %s", paths+fileName, clientIP), lab.Recipients...); err != nil {
				logger.Warningf("DownloadGitBlob", "", err, "failed to send notification for file \"%s\"", fileName)
			}
		}()
	}
	return
}

func (lab *HandleGitlabBrowser) MakeHandler(logger misc.Logger, cmdProc *common.CommandProcessor) (http.HandlerFunc, error) {
	fun := func(w http.ResponseWriter, r *http.Request) {
		shortcutName := strings.TrimSpace(r.FormValue("shortcut"))
		browsePath := r.FormValue("path")
		fileName := strings.TrimSpace(r.FormValue("file"))
		submitAction := r.FormValue("submit")

		NoCache(w)
		if !WarnIfNoHTTPS(r, w) {
			return
		}
		switch submitAction {
		case "Go":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			projectID, found := lab.Projects[shortcutName]
			if !found {
				w.Write([]byte(fmt.Sprintf(HandleGitlabPage, shortcutName, browsePath, fileName, "(cannot find shortcut name)")))
				return
			}
			dirs, fileNames, err := lab.ListGitObjects(projectID, browsePath)
			if err != nil {
				w.Write([]byte(fmt.Sprintf(HandleGitlabPage, shortcutName, browsePath, fileName, "Error: "+err.Error())))
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
			w.Write([]byte(fmt.Sprintf(HandleGitlabPage, shortcutName, browsePath, fileName, contentList)))
		case "Download":
			projectID, found := lab.Projects[shortcutName]
			if !found {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(fmt.Sprintf(HandleGitlabPage, shortcutName, browsePath, fileName, "(cannot find shortcut name)")))
				return
			}
			content, err := lab.DownloadGitBlob(logger, GetRealClientIP(r), projectID, browsePath, fileName)
			if err != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(fmt.Sprintf(HandleGitlabPage, shortcutName, browsePath, fileName, "Error: "+err.Error())))
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
			w.Write(content)
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(fmt.Sprintf(HandleGitlabPage, shortcutName, browsePath, fileName, "Enter path to browse or blob ID to download")))
		}
	}
	return fun, nil
}

func (_ *HandleGitlabBrowser) GetRateLimitFactor() int {
	return 5
}
