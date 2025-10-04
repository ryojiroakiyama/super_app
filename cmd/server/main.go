package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gmail-tts-app/internal/config"
    driveuploader "gmail-tts-app/internal/infrastructure/drive"
	"gmail-tts-app/internal/infrastructure/gmail"
	"gmail-tts-app/internal/infrastructure/googleauth"
	"gmail-tts-app/internal/infrastructure/storage"
	openaitts "gmail-tts-app/internal/infrastructure/tts/openai"
	usemsg "gmail-tts-app/internal/usecase/message"

	gmailapi "google.golang.org/api/gmail/v1"
    drivev3 "google.golang.org/api/drive/v3"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	log.Printf("[flow] starting run flow")

	// 1-2) Gmailアクセス可否を確認し、必要なら認証を促す
	srv, err := ensureGmailService(ctx)
	if err != nil {
		log.Printf("[auth] failed to obtain gmail service: %v", err)
		return
	}

	// 2.1) Driveアップロードが有効なら、必要に応じてDriveの認証も事前に促す
	if cfg.DriveUploadEnabled {
		log.Printf("[drive] preflight: ensuring Drive authorization")
		if _, err := ensureDriveService(ctx); err != nil {
			log.Printf("[drive] drive preflight failed: %v", err)
			return
		}
	}

	// 3) 既定条件で最新メールIDを取得（INBOXの最新1件、検索クエリ適用）
	q := getGmailQuery()
	if strings.TrimSpace(q) != "" {
		log.Printf("[gmail] applying query: %s", q)
	}
	msgID, err := latestMessageID(ctx, srv, q)
	if err != nil {
		log.Printf("[gmail] failed to get latest message id: %v", err)
		return
	}
	log.Printf("[gmail] latest message id: %s", msgID)

	// 4) downloaded_ids.txt に存在するか確認
	if alreadyDownloaded(msgID) {
		log.Printf("[flow] message %s already processed. exiting.", msgID)
		return
	}

	// 5) ダウンロード開始（本文取得→TTS→保存）
	repo := gmail.NewMessageRepository(srv)
	synth, err := openaitts.NewSynthesizer(cfg.OpenAIAPIKey)
	if err != nil {
		log.Printf("[tts] synthesizer init error: %v", err)
		return
	}
	store := storage.NewFileStore(cfg.AudioDir)
	uc := usemsg.NewGenerateAudioFromMessage(repo, synth, store)

	log.Printf("[flow] start synthesis for %s", msgID)
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	out, err := uc.Execute(ctxTimeout, &usemsg.GenerateAudioFromMessageInput{MessageID: msgID, LimitChars: 0})
	if err != nil {
		log.Printf("[flow] synthesis error: %v", err)
		return
	}
	log.Printf("[flow] saved audio to %s (bytes=%d)", out.LocalPath, len(out.Audio.Data))

    // 5.1) Optionally upload merged audio to Google Drive
    if cfg.DriveUploadEnabled {
        log.Printf("[drive] upload enabled. uploading to Drive folder=%s", cfg.DriveFolderID)
        if err := uploadToDrive(ctx, cfg, string(out.LocalPath)); err != nil {
            log.Printf("[drive] upload failed: %v", err)
        }
    }

	// 6) downloaded_ids.txt に追記
	if err := appendDownloadedID(msgID); err != nil {
		log.Printf("[flow] failed to append downloaded id: %v", err)
		return
	}
	log.Printf("[flow] completed for %s", msgID)
}

func ensureGmailService(ctx context.Context) (*gmailapi.Service, error) {
	// 試行: 既存トークンでアクセス可能か
	srv, err := googleauth.BuildGmailService(ctx)
	if err == nil {
		// 軽い疎通確認（レートに優しい範囲）
		if _, e := srv.Users.Labels.List("me").Context(ctx).Do(); e == nil {
			return srv, nil
		}
		// トークン不正と思われる場合は再認証
	}

	log.Printf("[auth] authorization required. starting interactive flow...")
	if e := googleauth.ObtainTokenInteractive(ctx); e != nil {
		return nil, e
	}
	return googleauth.BuildGmailService(ctx)
}

func latestMessageID(ctx context.Context, srv *gmailapi.Service, query string) (string, error) {
	call := srv.Users.Messages.List("me").LabelIds("INBOX").MaxResults(1).Context(ctx)
	if strings.TrimSpace(query) != "" {
		call = call.Q(query)
	}
	res, err := call.Do()
	if err != nil {
		return "", err
	}
	if res == nil || len(res.Messages) == 0 {
		return "", errors.New("no messages found in INBOX")
	}
	return res.Messages[0].Id, nil
}

func downloadedLogPath() string {
	return filepath.Join("log", "downloaded_ids.txt")
}

func alreadyDownloaded(id string) bool {
	path := downloadedLogPath()
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == id {
			return true
		}
	}
	return false
}

func appendDownloadedID(id string) error {
	path := downloadedLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", id)
	return err
}

func getGmailQuery() string {
    if v := os.Getenv("GMAIL_QUERY"); strings.TrimSpace(v) != "" {
        return v
    }
    // 既定の検索条件: 件名に「週刊Life is beautiful」
    return "subject:\"週刊Life is beautiful\""
}

// uploadToDrive uploads the given local file path to Drive. It tries existing token first.
func uploadToDrive(ctx context.Context, cfg *config.Config, localPath string) error {
    // Ensure service with current token and scopes
    srv, err := ensureDriveService(ctx)
    if err != nil {
        return err
    }
    uploader := driveuploader.NewUploader(srv)
    dstName := filepath.Base(localPath)

    id, link, err := uploader.UploadFile(ctx, localPath, dstName, cfg.DriveFolderID)
    if err == nil {
        log.Printf("[drive] uploaded: id=%s link=%s", id, link)
        return nil
    }
    // If failed, attempt interactive re-auth with Drive scope once
    log.Printf("[drive] upload error (%v). trying interactive auth...", err)
    if e := googleauth.ObtainTokenInteractiveWithScopes(ctx, gmailapi.GmailReadonlyScope, drivev3.DriveFileScope); e != nil {
        return e
    }
    // Build service again and retry once
    srv, err = googleauth.BuildDriveService(ctx)
    if err != nil {
        return err
    }
    uploader = driveuploader.NewUploader(srv)
    id, link, err = uploader.UploadFile(ctx, localPath, dstName, cfg.DriveFolderID)
    if err != nil {
        return err
    }
    log.Printf("[drive] uploaded: id=%s link=%s", id, link)
    return nil
}

func ensureDriveService(ctx context.Context) (*drivev3.Service, error) {
    // 既存トークンでDrive APIにアクセスできるか検証
    srv, err := googleauth.BuildDriveService(ctx)
    if err == nil {
        if _, e := srv.Files.List().PageSize(1).Context(ctx).Do(); e == nil {
            return srv, nil
        }
        // 権限不足などで失敗した場合は、DriveFileスコープを含めた対話認証を実施
        log.Printf("[drive] permission check failed. starting interactive auth for Drive...")
        if ie := googleauth.ObtainTokenInteractiveWithScopes(ctx, gmailapi.GmailReadonlyScope, drivev3.DriveFileScope); ie != nil {
            return nil, ie
        }
        // 再構築して再確認
        srv, err = googleauth.BuildDriveService(ctx)
        if err != nil {
            return nil, err
        }
        if _, e2 := srv.Files.List().PageSize(1).Context(ctx).Do(); e2 == nil {
            return srv, nil
        }
        return nil, fmt.Errorf("drive authorization failed")
    }

    // サービス構築自体に失敗した場合も、対話認証を試みる
    if ie := googleauth.ObtainTokenInteractiveWithScopes(ctx, gmailapi.GmailReadonlyScope, drivev3.DriveFileScope); ie != nil {
        return nil, ie
    }
    srv, err = googleauth.BuildDriveService(ctx)
    if err != nil {
        return nil, err
    }
    if _, e := srv.Files.List().PageSize(1).Context(ctx).Do(); e != nil {
        return nil, e
    }
    return srv, nil
}

// (bulk upload helper removed)
