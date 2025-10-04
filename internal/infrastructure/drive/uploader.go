package drive

import (
    "context"
    "errors"
    "fmt"
    "mime"
    "net/http"
    "os"
    "path/filepath"

    gdrive "google.golang.org/api/drive/v3"
    "google.golang.org/api/googleapi"
)

// Uploader uploads local files to Google Drive.
type Uploader struct {
    srv *gdrive.Service
}

func NewUploader(srv *gdrive.Service) *Uploader {
    return &Uploader{srv: srv}
}

// UploadFile uploads a file pointed by localPath to the Drive folder (folderID).
// dstFileName allows overriding the name. If empty, the base name of localPath is used.
// Returns fileID and webViewLink.
func (u *Uploader) UploadFile(ctx context.Context, localPath, dstFileName, folderID string) (string, string, error) {
    if dstFileName == "" {
        dstFileName = filepath.Base(localPath)
    }
    f, err := os.Open(localPath)
    if err != nil {
        return "", "", err
    }
    defer f.Close()

    mimeType := mime.TypeByExtension(filepath.Ext(dstFileName))
    if mimeType == "" {
        // fallback for mp3
        mimeType = "audio/mpeg"
    }

    file := &gdrive.File{
        Name:     dstFileName,
        MimeType: mimeType,
    }
    if folderID != "" {
        file.Parents = []string{folderID}
    }

    mediaOpts := []googleapi.MediaOption{googleapi.ChunkSize(2 * 1024 * 1024)}
    call := u.srv.Files.Create(file).Context(ctx).Media(f, mediaOpts...)
    created, err := call.Do()
    if err != nil {
        var gerr *googleapi.Error
        if ok := errors.As(err, &gerr); ok && gerr.Code == http.StatusConflict {
            // If a file with the same name exists, create anyway (Drive allows duplicates).
            // Just return the error as-is to let caller decide.
        }
        return "", "", fmt.Errorf("drive upload failed: %w", err)
    }

    // Request webViewLink via a get.
    got, err := u.srv.Files.Get(created.Id).Fields("id,webViewLink").Context(ctx).Do()
    if err != nil {
        return created.Id, "", nil
    }
    return got.Id, got.WebViewLink, nil
}


