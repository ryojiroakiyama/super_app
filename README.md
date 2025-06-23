# Gmail連携 × AI音声変換アプリ（Goベース）システム設計メモ

## 🎯 目的
Gmailの特定メールを取得し、AI音声に変換して再生するアプリを一般ユーザー向けに提供。

---

## ✅ 機能一覧

- GoogleアカウントでOAuth認証
- 特定条件のメール取得
- メール本文を音声（MP3）に変換
- 音声ファイルを保存＆再生
- 履歴管理

---

## 🏗️ システム構成（概要）

```
[ユーザー]
   ↓
[Web/モバイルアプリ]
   ↓
[API Gateway]
   ↓
[GoバックエンドAPI]
 ├─ Gmail API連携
 ├─ 音声変換（TTS）
 ├─ 音声保存（GCS）
 └─ メタ情報保存（DB）
```

---

## 🧱 技術スタック

| 目的         | 技術                         |
|--------------|------------------------------|
| APIサーバー   | Go (Fiber/Echo)               |
| メール取得     | Gmail API                     |
| 音声変換       | Google Cloud Text-to-Speech   |
| ファイル保存   | Google Cloud Storage          |
| DB            | Cloud SQL (MySQL/PostgreSQL) |
| フロント       | React / Flutter               |
| 認証          | Google OAuth2                 |

---

## 🔐 セキュリティ・考慮点

- OAuthスコープ制限（readonly）
- トークンの安全な保管
- 音声ファイルの署名付きURL発行
- レート制限対応

---

## 📈 将来的な拡張

- プッシュ通知（Pub/Sub連携）
- メールフィルター設定UI
- 多言語TTS対応
- オフライン再生
- サブスク課金対応（Stripeなど）

---

## 🔧 GoでのGmail API使用例（簡易）

```go
import (
    "context"
    "golang.org/x/oauth2/google"
    "google.golang.org/api/gmail/v1"
)

func getGmailService(token *oauth2.Token) *gmail.Service {
    config, _ := google.ConfigFromJSON(clientSecretJSON, gmail.GmailReadonlyScope)
    client := config.Client(context.Background(), token)
    service, _ := gmail.New(client)
    return service
}
```

---

## 🚀 MVPに必要なもの

- Goバックエンド（API）
- GCSバケット
- CloudSQL
- Gmail API & TTS API有効化
- Google OAuth2設定
- シンプルなWeb or モバイルUI
