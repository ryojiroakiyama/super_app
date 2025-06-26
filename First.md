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

1. `gmail_client.go` で保存済みトークンから Gmail Service を生成
2. エンドポイント
   - `GET /messages` : 一覧取得（クエリ）
     | パラメータ | 意味 | 既定 |
     |-----------|------|------|
     | `max`     | 最大取得件数            | `10` |
     | `q`       | Gmail 検索演算子をそのまま指定 | なし |
   - `GET /messages/latest` : `max=1` を固定し `q` も利用可（最新 1 件用）
3. ユニットテストでトークン読み込みとレスポンスパースを検証

👉 動作確認例
```bash
# 「週刊Life is beautiful」最新 1 通を取得
curl "http://localhost:8080/messages/latest?q=from:mailmag@mag2premium.com%20subject:%22週刊Life%20is%20beautiful%22" | jq
```
返却例
```json
{
  "messages": [
    {
      "id": "18c7f8...",
      "subject": "週刊Life is beautiful ...",
      "snippet": "（本文冒頭...)"
    }
  ]
}
```

---

## Step 4 : OpenAI Text-to-Speech (TTS) 連携

1. OpenAI ダッシュボードで **API キー** を発行
   - `.env` に `OPENAI_API_KEY=sk-...` を入れる **または** ルートに `openai_api_key.txt` を作成してキーを書き込む
2. `tts_handler.go` で `/tts` エンドポイントを実装（モデル: `tts-1`, デフォルト voice: `alloy`）
3. `POST /tts`（例）
   ```json
   {
     "text": "こんにちは OpenAI TTS",
     "voice": "alloy"  // 省略可
   }
   ```
   が Base64 MP3 を返す

👉 動作確認手順
```bash
# 2. サーバー起動
GOTOOLCHAIN=local go run .

# 3. 音声合成をリクエスト
curl -X POST http://localhost:8080/tts \
  -H "Content-Type: application/json" \
  -d '{"text":"Hello from OpenAI TTS"}' > resp.json

# 4. mp3 を再生
jq -r '.audioContent' resp.json | base64 -d > out.mp3
open out.mp3   # macOS の場合
```

---

## Step 5 : ローカル保存での結合テスト

1. `POST /messages/:id/tts` が以下を実行
   - Gmail からメールを FULL 取得し `text/plain`（無ければ `text/html`→タグ除去 or Snippet）を抽出
   - OpenAI TTS に渡して MP3 を生成
   - `./audios/<messageID>.mp3` に保存
   - JSON で `localPath` と `audioBase64` を返却

👉 動作確認手順
```bash
# サーバー起動
GOTOOLCHAIN=local go run .

# 最新メール ID を取得（例）
MSG_ID=$(curl -s \
  "http://localhost:8080/messages/latest?q=from:mailmag@mag2premium.com%20subject:%22週刊Life%20is%20beautiful%22" | jq -r '.messages[0].id')

# 本文 → 音声変換しレスポンスを保存, 500に限定
curl -X POST "http://localhost:8080/messages/$MSG_ID/tts?limit=500" -o response.json

# MP3 を生成して再生
jq -r '.audioBase64' response.json | base64 -d > out.mp3
open out.mp3   # macOS
```

`audios/` ディレクトリに `<id>.mp3` が保存され、音声が再生できれば OK です。

⚠️ **現状の制限**
冒頭500ならできるが、超えるとできなくなる。

---

## Step 6 : ストリーム再生対応（チャンク転送）

長文メール全文でもメモリ消費を抑えつつ、ブラウザや `curl` でリアルタイム再生できるようにします。

1. `/messages/:id/tts/stream` (GET) を追加
   - Gmail から本文を取得（Step 5 と同様）
   - OpenAI TTS を `{"stream": true}` で呼び出す
   - 生成された MP3 バイト列をチャンク転送 (`Content-Type: audio/mpeg`)
2. 4096 文字制限を回避するため、本文を 3,000 文字ごとに分割し、チャンクごとに順次リクエスト

👉 動作確認手順

```bash
# 1. サーバー起動
GOTOOLCHAIN=local go run .

# 2. メール ID を取得（例は最新 1 通）
MSG_ID=$(curl -s \
  "http://localhost:8080/messages/latest" | jq -r '.messages[0].id')

# 3. ブラウザで再生
open "http://localhost:8080/messages/${MSG_ID}/tts/stream"   # macOS

# もしくは curl でファイル保存
curl -L "http://localhost:8080/messages/${MSG_ID}/tts/stream?limit=1500" -o out.mp3
open out.mp3
```

> **チェックポイント**  
> ブラウザのネットワークタブで `Transfer-Encoding: chunked` が付いていることを確認し、再生が即時開始されれば成功。

---

## Step 7 : SQLite + GORM で履歴管理

1. `models.go` に `User`, `Message`, `AudioHistory` を定義
2. `repository.go` で簡易 CRUD 実装
3. 自動マイグレーションを有効化し、`GET /histories` で履歴一覧を返す

👉 変換のたびに履歴が登録され、取得できることを確認します。

---

## Step 8 : Cloud Storage アップロード & 署名付き URL 取得

1. GCS クライアントを追加し、Step 5 のローカル保存を GCS バケットに置き換え
2. アップロード後、署名付き URL（Signed URL）を生成し `audioURL` として返す

👉 ブラウザで URL にアクセスして音声が再生できるか確認します。

---

## Step 9 : まとめ & 自動テスト／Docker 化（サーバーのみ）

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