# サーバーサイド MVP 検証ステップ

README で定義した Gmail × AI 音声変換アプリの **ローカル検証** を、サーバーサイド（Go）のみで小さく積み上げる手順に分割しました。各ステップは 200〜400 行程度の実装量を目安にしています。

---

## Step 1 : Go プロジェクト雛形 & Healthcheck

1. `go mod init gmail-tts-app`
2. Echo もしくは Fiber で `main.go` を作成し、`GET /healthz` が 200 OK を返すだけのエンドポイントを実装
3. ホットリロード用に `air` などを導入（`make dev` で起動）

👉 まずは **サーバーが起動しヘルスチェックが通る** ことを確認します。

---

## Step 2 : Gmail API Quickstart（認証トークンの取得）

1. Google Cloud Console で OAuth クライアントを作成し、`credentials.json` を配置
2. `/auth/google` エンドポイントを作り、OAuth consent → `token.json` にリフレッシュトークンを保存
3. Google 提供の Quickstart コードを移植し、CLI ではなく HTTP ハンドラ内で動かす

👉 ブラウザで認可フローを完了し、ローカルにトークンが保存できるかを確認します。

---

## Step 3 : メール一覧取得 API

1. `gmail_client.go` を用意し、保存済みトークンから Gmail Service を生成
2. `GET /messages?max=10` で `MessageID / Subject / Snippet` を JSON で返す
3. ユニットテストでトークン読み込みとレスポンスパースを検証

👉 Postman や curl でメール一覧が取得できることをチェックします。

---

## Step 4 : Google Cloud Text-to-Speech (TTS) Quickstart

1. サービスアカウント JSON を取得し、`GOOGLE_APPLICATION_CREDENTIALS` を設定
2. `tts_client.go` で `Synthesize(text, lang, voice)` を実装
3. `POST /tts`（body: `text`）→ Base64 エンコードした MP3 バイト列を返す

👉 curl でテキストを送信し、返ってきた Base64 をデコード → 再生して音声を確認します。

---

## Step 5 : ローカル保存での結合テスト

1. `POST /messages/:id/tts` が以下を実行
   - Gmail から本文取得
   - TTS に渡して MP3 を生成
   - `./audios/<messageID>.mp3` に保存
   - JSON で `localPath` を返却

👉 生成された MP3 をローカルで再生し、メール → 音声変換の流れが動くか検証します。

---

## Step 6 : SQLite + GORM で履歴管理

1. `models.go` に `User`, `Message`, `AudioHistory` を定義
2. `repository.go` で簡易 CRUD 実装
3. 自動マイグレーションを有効化し、`GET /histories` で履歴一覧を返す

👉 変換のたびに履歴が登録され、取得できることを確認します。

---

## Step 7 : Cloud Storage アップロード & 署名付き URL 取得

1. GCS クライアントを追加し、Step 5 のローカル保存を GCS バケットに置き換え
2. アップロード後、署名付き URL（Signed URL）を生成し `audioURL` として返す

👉 ブラウザで URL にアクセスして音声が再生できるか確認します。

---

## Step 8 : まとめ & 自動テスト／Docker 化（サーバーのみ）

1. `docker-compose.yml` を用意し、`app`, `sqlite`（必要なら `gcloud-sdk`）を定義
2. `Dockerfile`（multi-stage build）でアプリをビルド
3. GitHub Actions で `go test ./...` と `docker build` を実行
4. README にローカル検証手順を追記

👉 `docker compose up -d` で一式立ち上げ、E2E テストが通るところまでをゴールにします。

---

### 進め方のヒント

- **各ステップ完了ごとに Git commit** して小さい差分を保つ
- **単体テスト → 結合テスト → E2E** と段階的にカバレッジを広げる
- README とこのファイル（First.md）を随時更新し、学びを残す

これでサーバーサイド MVP の機能を安全に積み上げながら検証できます。 