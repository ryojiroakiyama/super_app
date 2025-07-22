# スマホアプリ開発ロードマップ

Gmail連携 × AI音声変換アプリのモバイル版実現方針

**本ドキュメントの目的**: AIアシスタントが段階的にモバイルアプリを実装するための詳細なガイド

---

## 📋 プロジェクト概要

### 現在の状況
- **リポジトリ**: `/Users/ryojiro_akiyama/workspace/personal/super_app`
- **現在のブランチ**: `main` (origin/mainより1コミット先行)
- **実装済み機能**: Webベースの Gmail × OpenAI TTS 連携アプリ

### 目標
既存のGoバックエンドを活用し、Flutter モバイルアプリを開発する

---

## 📱 現在のシステム詳細分析

### 主要機能
- **Gmailからメール取得・一覧表示・フィルタリング**
  - Google OAuth2認証によるセキュアなアクセス
  - Gmail API経由でのメール検索・取得
  - フィルター機能（From、Subject条件指定）
- **OpenAI TTSでメール本文を音声変換**
  - テキストの分割処理（3000文字単位）
  - ストリーミング再生対応
  - MP3形式での音声生成
- **音声再生・ダウンロード機能**
  - リアルタイムストリーミング再生
  - ローカルファイル保存・ダウンロード
  - 音声履歴管理

### 技術構成詳細

#### バックエンド (継続利用)
- **フレームワーク**: Go 1.21 + Fiber v2.52.8
- **アーキテクチャ**: ドメイン駆動設計 (DDD)
- **主要パッケージ**:
  ```
  - google.golang.org/api (Gmail API)
  - golang.org/x/oauth2 (認証)
  - github.com/gofiber/fiber/v2 (HTTP フレームワーク)
  ```
- **認証**: Google OAuth2 (credentials.json + token.json)
- **音声合成**: OpenAI TTS API (tts-1 model, alloy voice)
- **ストレージ**: ローカルファイル (`./audio/` ディレクトリ)

#### API エンドポイント (現在)
```
GET  /healthz                     - ヘルスチェック
GET  /auth/google                 - OAuth認証開始
GET  /auth/callback               - OAuth コールバック
GET  /messages                    - メール一覧取得
POST /messages/:id/tts            - 音声生成 (同期)
GET  /messages/:id/tts/stream     - 音声ストリーミング
GET  /audios/merged/:id.mp3       - 音声ファイル配信
```

#### フロントエンド (現在 - 参考実装)
- **技術**: Vanilla JS + Tailwind CSS
- **ファイル**: 
  - `public/index.html` - メイン画面
  - `public/main.js` - フロントエンドロジック

---

## 🎯 実装戦略

### 採用アプローチ: **API-First ハイブリッド開発**

**戦略**: 既存GoバックエンドのAPIを拡張し、Flutter モバイルアプリを新規開発

### 実装判断の根拠
1. **既存資産活用**: `internal/` の実装済みドメインロジックを継続利用
2. **開発効率**: Gmail API・OpenAI TTS 連携の再実装不要
3. **品質保証**: 実績のある認証・セキュリティ実装を維持
4. **技術適合**: GoのAPIとFlutterの相性が良好

### 技術選定詳細

#### モバイルフレームワーク: Flutter
**選定理由**:
- iOS/Android 同時開発
- 高いパフォーマンス (Dart言語)
- 豊富な音声・HTTP ライブラリ
- Google OAuth2 サポート充実

#### 推奨ライブラリスタック
```yaml
dependencies:
  flutter: sdk
  dio: ^5.0.0                    # HTTP クライアント
  riverpod: ^2.0.0               # 状態管理
  just_audio: ^0.9.0             # 音声再生
  sqflite: ^2.0.0                # ローカルDB
  google_sign_in: ^6.0.0         # Google OAuth2
  firebase_messaging: ^14.0.0    # プッシュ通知
```

### システム構成
```
┌─────────────────────────────────────┐
│  Flutter Mobile App                 │
│  ├─ UI Layer (Widgets)              │
│  ├─ State Management (Riverpod)     │
│  ├─ Data Layer (Repository)         │
│  └─ Local Storage (SQLite)          │
└─────────────┬───────────────────────┘
              │ HTTP/REST API
┌─────────────▼───────────────────────┐
│  Existing Go Backend (継続利用)      │
│  ├─ cmd/server/main.go              │
│  ├─ internal/usecase/               │
│  ├─ internal/infrastructure/        │
│  │   ├─ gmail/                      │
│  │   ├─ tts/openai/                 │
│  │   └─ storage/                    │
│  └─ internal/interface/http/        │
└─────────────┬───────────────────────┘
              │
┌─────────────▼───────────────────────┐
│  External APIs                      │
│  ├─ Gmail API                       │
│  ├─ OpenAI TTS API                  │
│  └─ Google OAuth2                   │
└─────────────────────────────────────┘
```

---

## 🗺️ 実現までのステップ

### **Phase 1: モバイル対応API基盤整備** (2-3週間)

**目標**: 既存Goバックエンドをモバイルアプリから効率的に利用できるAPIに拡張

#### Step 1-1: API認証システム拡張
**実装場所**: `internal/interface/http/handler/`
**成果物**: モバイル認証対応のエンドポイント群

**具体的実装タスク**:

1. **JWT認証ミドルウェア追加**
   ```go
   // internal/infrastructure/auth/jwt.go 新規作成
   - JWTトークン生成・検証機能
   - リフレッシュトークン管理
   - トークン有効期限管理 (7日/30日)
   ```

2. **認証エンドポイント拡張**
   ```go
   // internal/interface/http/handler/auth_handler.go 新規作成
   POST /api/v1/auth/mobile/login    - モバイル向けOAuth開始
   POST /api/v1/auth/mobile/callback - OAuth完了 + JWT発行
   POST /api/v1/auth/refresh         - JWT リフレッシュ
   POST /api/v1/auth/logout          - ログアウト処理
   GET  /api/v1/auth/me              - ユーザー情報取得
   ```

3. **設定ファイル更新**
   ```yaml
   # internal/config/config.go に追加
   - JWT_SECRET_KEY
   - JWT_ACCESS_EXPIRE_HOURS
   - JWT_REFRESH_EXPIRE_DAYS
   ```

#### Step 1-2: メール API のモバイル最適化
**実装場所**: `internal/interface/http/handler/message_handler.go`
**前提**: Step 1-1 のJWT認証が完了していること

**具体的実装タスク**:

1. **ページネーション対応**
   ```go
   // 既存の GET /messages を拡張
   GET /api/v1/messages?cursor={base64_cursor}&limit={int}&q={query}
   
   Response:
   {
     "messages": [...],
     "nextCursor": "eyJ0aW1lc3RhbXAiOjE2ODc...",
     "hasMore": true
   }
   ```

2. **メール詳細API統一**
   ```go
   // 新規エンドポイント
   GET /api/v1/messages/{id}
   
   Response:
   {
     "id": "...",
     "subject": "...",
     "from": "...",
     "date": "2024-01-01T00:00:00Z",
     "body": "...",
     "preview": "...",
     "hasAudio": true,
     "audioGenerated": "2024-01-01T00:00:00Z"
   }
   ```

3. **検索機能強化**
   ```go
   GET /api/v1/messages/search?q={query}&filter={preset}&sort={field}
   
   // プリセットフィルター対応
   - recent: 最近1週間
   - important: 重要なメール
   - unread: 未読
   ```

#### Step 1-3: 音声管理API新規開発
**実装場所**: `internal/interface/http/handler/audio_handler.go` (新規)
**前提**: 音声ファイル管理のドメインロジック実装

**具体的実装タスク**:

1. **音声ジョブ管理システム**
   ```go
   // internal/domain/audio/job.go 新規作成
   POST /api/v1/audio/jobs           - 音声生成ジョブ作成
   GET  /api/v1/audio/jobs/{id}      - ジョブ状況取得
   GET  /api/v1/audio/jobs           - ジョブ一覧
   
   Job Status: pending, processing, completed, failed
   ```

2. **音声ファイル管理**
   ```go
   GET    /api/v1/audio/files        - 音声ファイル一覧
   GET    /api/v1/audio/files/{id}   - 音声ファイル詳細
   DELETE /api/v1/audio/files/{id}   - 音声ファイル削除
   GET    /api/v1/audio/stream/{id}  - 音声ストリーミング (既存機能)
   ```

3. **音声履歴・統計**
   ```go
   GET /api/v1/audio/history         - 再生履歴
   POST /api/v1/audio/history        - 再生記録
   GET /api/v1/audio/stats           - 利用統計
   ```

#### Step 1-4: モバイル特化機能追加
**実装場所**: 新規パッケージ `internal/infrastructure/notification/`, `internal/interface/http/handler/`
**前提**: Step 1-1〜1-3 が完了していること

**具体的実装タスク**:

1. **プッシュ通知基盤構築**
   ```go
   // internal/infrastructure/notification/fcm.go 新規作成
   - Firebase Admin SDK 統合
   - デバイストークン管理
   - 通知テンプレート管理
   
   // API エンドポイント
   POST /api/v1/notifications/register   - デバイストークン登録
   POST /api/v1/notifications/settings   - 通知設定更新
   GET  /api/v1/notifications/history    - 通知履歴
   ```

2. **オフライン対応API**
   ```go
   // メタデータ同期
   GET /api/v1/sync/messages/metadata    - メール一覧のメタデータのみ
   GET /api/v1/sync/status               - 同期状態確認
   POST /api/v1/sync/conflicts           - 競合解決
   
   // 音声プリダウンロード
   POST /api/v1/audio/preload            - 音声ファイル事前生成
   GET  /api/v1/audio/availability       - オフライン利用可能ファイル一覧
   ```

3. **ユーザー設定管理**
   ```go
   // internal/domain/user/settings.go 新規作成
   GET    /api/v1/users/settings         - 設定取得
   PUT    /api/v1/users/settings         - 設定更新
   POST   /api/v1/users/settings/backup  - 設定バックアップ作成
   POST   /api/v1/users/settings/restore - 設定復元
   
   // 設定項目
   {
     "audio": {
       "voice": "alloy",
       "speed": 1.0,
       "autoDownload": true
     },
     "filters": {
       "defaultQuery": "from:important@example.com",
       "presets": [...]
     },
     "notifications": {
       "newMails": true,
       "audioReady": true,
       "quietHours": "22:00-07:00"
     }
   }
   ```

#### Step 1-5: セキュリティ・パフォーマンス強化
**実装場所**: `cmd/server/main.go`, `internal/infrastructure/middleware/`
**前提**: 全てのAPI実装が完了していること

**具体的実装タスク**:

1. **セキュリティミドルウェア追加**
   ```go
   // internal/infrastructure/middleware/ に新規作成
   - rate_limiter.go     - レート制限 (100req/min per user)
   - cors.go             - CORS設定 (モバイルアプリドメイン許可)
   - security_headers.go - セキュリティヘッダー
   - request_logger.go   - API アクセスログ
   ```

2. **API応答最適化**
   ```go
   // cmd/server/main.go に追加
   - Gzip圧縮ミドルウェア
   - ETag設定 (キャッシュ制御)
   - 応答時間監視
   ```

3. **設定ファイル更新**
   ```go
   // internal/config/config.go に追加
   type Config struct {
     // ... 既存設定
     Security SecurityConfig
     Cache    CacheConfig
   }
   
   type SecurityConfig struct {
     RateLimit      int
     AllowedOrigins []string
     CSPPolicy      string
   }
   ```

**Phase 1 完了条件チェックリスト**:
- [ ] JWT認証が動作する
- [ ] `/api/v1/` プレフィックスでAPIが統一されている
- [ ] ページネーション付きでメール一覧が取得できる
- [ ] 音声ジョブ管理が動作する
- [ ] プッシュ通知送信テストが成功する
- [ ] レート制限が適切に動作する

---

### **Phase 2: Flutter モバイルアプリ開発** (4-6週間)

**目標**: Phase 1 で構築したAPIを活用して、iOS/Android アプリを開発
**前提**: Phase 1 のAPI基盤が完了していること

#### Step 2-1: Flutter プロジェクト初期化
**作業場所**: 新規ディレクトリ `mobile_app/` (リポジトリルート下)
**成果物**: 動作する Flutter アプリの雛形

**具体的実装タスク**:

1. **Flutter プロジェクト作成**
   ```bash
   # リポジトリルート下で実行
   flutter create --org com.super_app --project-name gmail_tts_mobile mobile_app
   cd mobile_app
   ```

2. **依存関係設定**
   ```yaml
   # mobile_app/pubspec.yaml
   dependencies:
     flutter:
       sdk: flutter
     
     # HTTP & 状態管理
     dio: ^5.4.0
     riverpod: ^2.4.0
     flutter_riverpod: ^2.4.0
     
     # 認証
     google_sign_in: ^6.2.1
     flutter_secure_storage: ^9.0.0
     
     # 音声
     just_audio: ^0.9.36
     audio_service: ^0.18.12
     
     # ローカルDB
     sqflite: ^2.3.0
     
     # プッシュ通知
     firebase_core: ^2.24.2
     firebase_messaging: ^14.7.10
     
     # UI
     flutter_localizations:
       sdk: flutter
     intl: ^0.19.0
   
   dev_dependencies:
     flutter_test:
       sdk: flutter
     flutter_lints: ^3.0.0
     build_runner: ^2.4.7
   ```

3. **プロジェクト構造設計**
   ```
   mobile_app/
   lib/
   ├─ main.dart                     # エントリーポイント
   ├─ app/
   │  ├─ app.dart                   # MaterialApp設定
   │  └─ router.dart                # ナビゲーション設定
   ├─ core/
   │  ├─ constants/                 # 定数定義
   │  ├─ errors/                    # エラー処理
   │  ├─ network/                   # HTTP クライアント
   │  └─ storage/                   # ローカルストレージ
   ├─ features/
   │  ├─ auth/                      # 認証機能
   │  │  ├─ data/                   # データレイヤー
   │  │  ├─ domain/                 # ドメインレイヤー
   │  │  └─ presentation/           # プレゼンテーションレイヤー
   │  ├─ messages/                  # メール機能
   │  ├─ audio/                     # 音声機能
   │  └─ settings/                  # 設定機能
   └─ shared/
      ├─ models/                    # 共通データモデル
      ├─ providers/                 # Riverpod プロバイダー
      ├─ services/                  # 共通サービス
      └─ widgets/                   # 共通ウィジェット
   ```

4. **開発環境設定**
   ```bash
   # analysis_options.yaml 設定
   # .gitignore 更新 (iOS/Android固有ファイル除外)
   # VSCode設定 (.vscode/settings.json)
   ```

#### Step 2-2: 認証機能実装
**実装場所**: `lib/features/auth/`
**前提**: Phase 1 の認証APIが動作していること

**具体的実装タスク**:

1. **認証データモデル**
   ```dart
   // lib/features/auth/domain/entities/user.dart
   class User {
     final String id;
     final String email;
     final String displayName;
     final String? photoUrl;
   }
   
   // lib/features/auth/domain/entities/auth_tokens.dart
   class AuthTokens {
     final String accessToken;
     final String refreshToken;
     final DateTime expiresAt;
   }
   ```

2. **認証リポジトリ実装**
   ```dart
   // lib/features/auth/data/repositories/auth_repository_impl.dart
   class AuthRepositoryImpl implements AuthRepository {
     // Google Sign In 実装
     Future<User> signInWithGoogle()
     
     // JWT トークン管理
     Future<void> saveTokens(AuthTokens tokens)
     Future<AuthTokens?> getStoredTokens()
     Future<void> refreshTokens()
     Future<void> signOut()
   }
   ```

3. **認証画面UI**
   ```dart
   // lib/features/auth/presentation/pages/login_page.dart
   class LoginPage extends ConsumerWidget {
     // Google ログインボタン
     // ロゴ・アプリ説明
     // 利用規約リンク
   }
   ```

#### Step 2-3: メール機能実装
**実装場所**: `lib/features/messages/`
**前提**: Step 2-2 の認証が完了していること

**具体的実装タスク**:

1. **メールデータモデル**
   ```dart
   // lib/features/messages/domain/entities/email_message.dart
   class EmailMessage {
     final String id;
     final String subject;
     final String from;
     final DateTime date;
     final String preview;
     final bool hasAudio;
     final DateTime? audioGeneratedAt;
   }
   
   // lib/features/messages/domain/entities/message_list.dart
   class MessageList {
     final List<EmailMessage> messages;
     final String? nextCursor;
     final bool hasMore;
   }
   ```

2. **メールリポジトリ実装**
   ```dart
   // lib/features/messages/data/repositories/message_repository_impl.dart
   class MessageRepositoryImpl implements MessageRepository {
     Future<MessageList> getMessages({
       String? cursor,
       int limit = 20,
       String? query,
     })
     
     Future<EmailMessage> getMessageDetail(String id)
     Future<MessageList> searchMessages(String query)
   }
   ```

3. **メール一覧画面**
   ```dart
   // lib/features/messages/presentation/pages/message_list_page.dart
   class MessageListPage extends ConsumerWidget {
     // Pull-to-refresh 対応
     // 無限スクロール実装
     // 検索・フィルター機能
     // メッセージプレビューカード
   }
   ```

#### Step 2-4: 音声機能実装
**実装場所**: `lib/features/audio/`
**前提**: Step 2-3 のメール機能が完了していること

**具体的実装タスク**:

1. **音声データモデル**
   ```dart
   // lib/features/audio/domain/entities/audio_job.dart
   class AudioJob {
     final String id;
     final String messageId;
     final AudioJobStatus status;
     final DateTime createdAt;
     final String? errorMessage;
   }
   
   enum AudioJobStatus { pending, processing, completed, failed }
   ```

2. **音声プレイヤー実装**
   ```dart
   // lib/features/audio/presentation/providers/audio_player_provider.dart
   class AudioPlayerProvider extends StateNotifier<AudioPlayerState> {
     // just_audio 実装
     Future<void> playStreamingAudio(String messageId)
     Future<void> downloadAndPlay(String messageId)
     void pause()
     void resume()
     void seek(Duration position)
   }
   ```

3. **音声プレイヤーUI**
   ```dart
   // lib/features/audio/presentation/widgets/audio_player_widget.dart
   class AudioPlayerWidget extends ConsumerWidget {
     // 再生/一時停止ボタン
     // プログレスバー
     // 再生速度調整
     // ダウンロードボタン
   }
   ```

#### Step 2-5: 設定・通知機能実装
**実装場所**: `lib/features/settings/`, `lib/features/notifications/`
**前提**: Step 2-4 の音声機能が完了していること

**具体的実装タスク**:

1. **設定データモデル・管理**
   ```dart
   // lib/features/settings/domain/entities/app_settings.dart
   class AppSettings {
     final AudioSettings audio;
     final FilterSettings filters;
     final NotificationSettings notifications;
     final DisplaySettings display;
   }
   
   // lib/features/settings/presentation/pages/settings_page.dart
   class SettingsPage extends ConsumerWidget {
     // 音声設定 (voice, speed, auto-download)
     // フィルター設定 (プリセット管理)
     // 通知設定 (新着メール, 音声完了)
     // 表示設定 (ダークモード, 言語)
   }
   ```

2. **プッシュ通知実装**
   ```dart
   // lib/core/services/notification_service.dart
   class NotificationService {
     Future<void> initialize()
     Future<void> registerDeviceToken()
     Future<void> handleBackgroundMessage(RemoteMessage message)
     Future<void> showLocalNotification(String title, String body)
   }
   
   // Firebase設定
   // android/app/google-services.json
   // ios/Runner/GoogleService-Info.plist
   ```

3. **オフライン機能実装**
   ```dart
   // lib/core/services/sync_service.dart
   class SyncService {
     Future<void> syncMessagesMetadata()
     Future<void> predownloadAudio(List<String> messageIds)
     Future<bool> isMessageAvailableOffline(String messageId)
     Future<void> clearOfflineData()
   }
   ```

**Phase 2 完了条件チェックリスト**:
- [ ] Google OAuth2ログインが動作する
- [ ] メール一覧の取得・表示ができる
- [ ] 音声生成・ストリーミング再生ができる
- [ ] オフライン機能が動作する
- [ ] プッシュ通知が受信できる
- [ ] iOS/Android両方でビルドできる

---

### **Phase 3: 本番インフラ・リリース準備** (2-3週間)

**目標**: 本番環境構築とストア配信準備を完了
**前提**: Phase 2 のモバイルアプリが動作していること

#### Step 3-1: バックエンド本番環境構築
**実装場所**: インフラ設定ファイル、デプロイスクリプト
**成果物**: 本番で稼働するGoバックエンド環境

**具体的実装タスク**:

1. **Google Cloud Platform 移行**
   ```bash
   # プロジェクト設定
   gcloud config set project gmail-tts-mobile-prod
   
   # Cloud SQL インスタンス作成
   gcloud sql instances create gmail-tts-db \
     --database-version=POSTGRES_15 \
     --tier=db-f1-micro \
     --region=asia-northeast1
   
   # Cloud Storage バケット作成
   gsutil mb gs://gmail-tts-audio-files
   gsutil iam ch allUsers:objectViewer gs://gmail-tts-audio-files
   ```

2. **Docker化・Cloud Run デプロイ**
   ```dockerfile
   # Dockerfile (リポジトリルート)
   FROM golang:1.21-alpine AS builder
   WORKDIR /app
   COPY go.mod go.sum ./
   RUN go mod download
   COPY . .
   RUN go build -o server ./cmd/server
   
   FROM alpine:latest
   RUN apk --no-cache add ca-certificates
   WORKDIR /root/
   COPY --from=builder /app/server .
   EXPOSE 8080
   CMD ["./server"]
   ```

3. **環境変数・シークレット管理**
   ```bash
   # Secret Manager設定
   gcloud secrets create openai-api-key --data-file=secrets/openai_api_key.txt
   gcloud secrets create jwt-secret --data-file=secrets/jwt_secret.txt
   
   # Cloud Run デプロイ
   gcloud run deploy gmail-tts-api \
     --image=gcr.io/gmail-tts-mobile-prod/api:latest \
     --platform=managed \
     --region=asia-northeast1 \
     --set-env-vars="DATABASE_URL=postgres://..." \
     --set-secrets="OPENAI_API_KEY=openai-api-key:latest"
   ```

#### Step 3-2: アプリストア配信準備
**実装場所**: `mobile_app/android/`, `mobile_app/ios/`
**成果物**: ストア申請可能なアプリパッケージ

**具体的実装タスク**:

1. **iOS配信準備**
   ```bash
   # Apple Developer Program 設定
   # - Team ID: XXXXXXXXXX
   # - App ID: com.super_app.gmail_tts_mobile
   # - Capabilities: Push Notifications, Background App Refresh
   
   # ios/Runner/Info.plist 設定
   <key>NSMicrophoneUsageDescription</key>
   <string>音声再生機能のため</string>
   <key>NSLocationWhenInUseUsageDescription</key>
   <string>プッシュ通知のため</string>
   
   # ビルド・アーカイブ
   cd mobile_app
   flutter build ios --release
   # Xcode でアーカイブ・App Store Connect アップロード
   ```

2. **Android配信準備**
   ```bash
   # Google Play Console 設定
   # - Package name: com.super_app.gmail_tts_mobile
   # - Signing key: 自動管理
   
   # android/app/build.gradle 設定
   android {
     compileSdkVersion 34
     defaultConfig {
       minSdkVersion 21
       targetSdkVersion 34
       versionCode 1
       versionName "1.0.0"
     }
   }
   
   # AAB ビルド
   flutter build appbundle --release
   ```

3. **ストア素材準備**
   ```bash
   # アプリアイコン生成 (1024x1024 から各サイズ)
   # スクリーンショット作成
   # - iPhone: 6.7", 6.5", 5.5"
   # - Android: Phone, Tablet
   
   # ストア説明文 (日本語・英語)
   # プライバシーポリシー URL
   # 利用規約 URL
   ```

#### Step 3-3: テスト・品質保証実装
**実装場所**: `mobile_app/test/`, CI/CD設定
**成果物**: 自動テストスイート、品質保証プロセス

**具体的実装タスク**:

1. **自動テスト実装**
   ```dart
   // test/unit_test.dart - 単体テスト
   group('AuthRepository', () {
     test('should save tokens correctly', () async {
       // JWT トークン保存テスト
     });
   });
   
   // test/widget_test.dart - ウィジェットテスト
   group('LoginPage Widget', () {
     testWidgets('should show login button', (tester) async {
       // ログイン画面テスト
     });
   });
   
   // integration_test/app_test.dart - 結合テスト
   group('Full App Test', () {
     testWidgets('complete user flow', (tester) async {
       // ログイン→メール一覧→音声再生のフローテスト
     });
   });
   ```

2. **CI/CD パイプライン**
   ```yaml
   # .github/workflows/test.yml
   name: Test
   on: [push, pull_request]
   jobs:
     test:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v3
         - uses: subosito/flutter-action@v2
         - run: flutter test
         - run: flutter analyze
   
   # .github/workflows/build.yml
   name: Build
   on:
     push:
       tags: ['v*']
   jobs:
     build-android:
       runs-on: ubuntu-latest
       steps:
         - run: flutter build appbundle --release
     build-ios:
       runs-on: macos-latest
       steps:
         - run: flutter build ios --release --no-codesign
   ```

3. **デバイステスト設定**
   ```bash
   # Firebase Test Lab設定
   gcloud firebase test android run \
     --type=robo \
     --app=build/app/outputs/bundle/release/app-release.aab \
     --device=model=Pixel2,version=28,locale=ja,orientation=portrait
   
   # iOS実機テスト (TestFlight)
   # - 内部テスター登録
   # - ベータ版配信
   # - フィードバック収集システム
   ```

**Phase 3 完了条件チェックリスト**:
- [ ] 本番環境でAPIが正常動作する
- [ ] Cloud Storage に音声ファイルが保存される
- [ ] iOS TestFlight でアプリが配信できる
- [ ] Google Play Console でベータ版が配信できる
- [ ] 自動テストが全て通る
- [ ] パフォーマンステストで基準を満たす

---

### **Phase 4: リリース・運用・継続改善** (継続的)

**目標**: 安定運用とユーザー価値向上の継続的実現
**前提**: Phase 3 でストア配信準備が完了していること

#### Step 4-1: 初回リリース・運用体制構築
**実装場所**: 運用ツール設定、監視システム
**成果物**: 本番運用中のモバイルアプリ

**具体的実装タスク**:

1. **段階的リリース実行**
   ```bash
   # ベータ版リリース (TestFlight / Play Console Internal Testing)
   # 限定ユーザー数: 50-100名
   # テスト期間: 1-2週間
   # フィードバック収集・修正
   
   # 本番リリース準備
   # - App Store Review 申請
   # - Google Play Review 申請
   # - リリースノート作成
   
   # 段階的ロールアウト
   # - 初期: 5% ユーザー
   # - 1週間後: 25% ユーザー
   # - 2週間後: 100% ユーザー
   ```

2. **運用監視システム構築**
   ```go
   // 監視・アラート設定
   // internal/infrastructure/monitoring/
   
   - crashlytics.go          # クラッシュレポート監視
   - analytics.go            # ユーザー行動分析
   - performance_monitor.go  # パフォーマンス監視
   - alert_manager.go        # アラート管理
   
   // 監視項目
   - API レスポンス時間 (95%ile < 500ms)
   - エラー率 (< 1%)
   - アプリクラッシュ率 (< 0.1%)
   - 音声生成成功率 (> 95%)
   ```

3. **ユーザーサポート体制**
   ```markdown
   ## サポート体制構築
   
   ### FAQ作成
   - ログインできない場合の対処法
   - 音声が再生されない場合
   - オフライン機能の使い方
   - 通知が来ない場合の設定
   
   ### サポート窓口
   - アプリ内フィードバック機能
   - メールサポート (support@super-app.com)
   - 公式サイト FAQ ページ
   
   ### エスカレーション手順
   - レベル1: FAQ・自動回答
   - レベル2: 技術サポート担当
   - レベル3: 開発チーム
   ```

#### Step 4-2: データ分析・改善サイクル構築
**実装場所**: 分析基盤、A/Bテスト仕組み
**成果物**: データドリブンな改善プロセス

**具体的実装タスク**:

1. **ユーザー行動分析実装**
   ```dart
   // lib/core/analytics/analytics_service.dart
   class AnalyticsService {
     // ユーザー行動追跡
     void trackScreenView(String screenName)
     void trackButtonTap(String buttonName)
     void trackAudioPlayback(String messageId, Duration duration)
     void trackError(String errorType, String details)
     
     // カスタムイベント
     void trackAudioGeneration(String messageId, bool success)
     void trackOfflineUsage(int cachedMessagesCount)
   }
   ```

2. **A/Bテスト基盤**
   ```dart
   // lib/core/experiments/ab_test_service.dart
   class ABTestService {
     // 実験管理
     bool isExperimentEnabled(String experimentName)
     String getVariant(String experimentName)
     void trackConversion(String experimentName, String event)
     
     // 実験例
     - audio_player_ui_v2      # プレイヤーUI改善
     - onboarding_flow_v3      # オンボーディング最適化
     - notification_frequency  # 通知頻度調整
   }
   ```

#### Step 4-3: 機能拡張ロードマップ実装
**実装場所**: 既存コードベース拡張
**成果物**: ユーザー価値向上機能

**具体的実装タスク**:

1. **音声機能強化 (v1.1.0)**
   ```go
   // internal/infrastructure/tts/ に追加
   
   - azure/synthesizer.go     # Azure Cognitive Services TTS
   - aws/synthesizer.go       # AWS Polly TTS
   - voice_selector.go        # 音声プロバイダー選択ロジック
   
   // 新機能
   - 音声品質選択 (standard/premium)
   - 感情表現対応 (喜び、悲しみ、怒り)
   - 音声ブックマーク機能
   - 音声要約 (AI活用)
   ```

2. **AI機能拡張 (v1.2.0)**
   ```go
   // internal/domain/ai/ 新規パッケージ
   
   - summarizer.go            # メール内容要約
   - importance_classifier.go # 重要度判定
   - category_classifier.go   # カテゴリー自動分類
   - filter_recommender.go    # フィルター提案
   
   // OpenAI GPT-4 活用
   - メール内容の要約生成
   - 重要なメールの自動検出
   - カテゴリー自動分類
   ```

3. **プラットフォーム拡張 (v1.3.0)**
   ```dart
   // ウィジェット対応
   // ios/Widget/TodayExtension/
   // android/app/src/main/widget/
   
   - 最新メール表示ウィジェット
   - 音声再生コントロールウィジェット
   - Apple Watch アプリ (WatchKit)
   - タブレット UI最適化
   ```

**継続改善KPI**:
- 月間アクティブユーザー (MAU)
- 音声生成使用率
- アプリクラッシュ率
- ユーザー満足度 (アプリストアレビュー)
- 音声再生完了率

**リリースサイクル**:
- **マイナーアップデート**: 2週間ごと (バグ修正、小改善)
- **機能アップデート**: 1ヶ月ごと (新機能追加)
- **メジャーアップデート**: 3ヶ月ごと (大きな機能改善)

**長期ロードマップ (6ヶ月〜1年)**:
- 多言語対応 (英語、中国語、韓国語)
- 企業向け機能 (チーム共有、管理機能)
- サブスクリプション機能
- 外部サービス連携 (Slack, Teams, Notion)
- オフライン AI 機能 (デバイス上での音声生成)

---

## 💡 実装上の重要ポイント

### バックエンド継続利用の利点
- ✅ **既存資産活用**: Gmail API・OpenAI TTS連携をそのまま利用
- ✅ **ドメインロジック保持**: 音声変換・メール処理ロジックの再実装不要
- ✅ **認証基盤活用**: OAuth2認証フローの継続利用
- ✅ **セキュリティ**: 実績のあるセキュリティ実装を維持

### モバイルアプリ独自の考慮点
- 📱 **タッチ最適化**: スワイプ・ピンチ・長押し等のタッチ操作対応
- 🔋 **省電力設計**: バックグラウンド処理最適化・バッテリー消費削減
- 📶 **オフライン対応**: ネットワーク不安定時の対応・ローカルキャッシュ活用
- 🔔 **プッシュ通知**: 適切なタイミングでの通知・ユーザー体験向上
- 🎵 **バックグラウンド再生**: 他アプリ使用中の音声再生継続
- 🔒 **モバイルセキュリティ**: デバイス固有のセキュリティ対応・生体認証活用

### パフォーマンス最適化
- ⚡ **レスポンシブ設計**: 60fps維持・スムーズなアニメーション
- 🗄️ **効率的キャッシュ**: メタデータ・音声ファイルの戦略的キャッシュ
- 🌐 **ネットワーク最適化**: リクエスト最小化・レスポンス圧縮
- 💾 **メモリ管理**: 大容量音声ファイルの効率的メモリ利用

---

## 📅 実装スケジュール詳細

| フェーズ | 期間 | 主要成果物 | 検証方法 |
|---------|------|-----------|----------|
| **Phase 1** | 2-3週間 | モバイル対応API群 | Postman/curl でAPI動作確認 |
| **Phase 2** | 4-6週間 | Flutter アプリMVP | iOS/Android実機動作確認 |
| **Phase 3** | 2-3週間 | 本番環境・ストアパッケージ | TestFlight/Internal Testing 配信 |
| **Phase 4** | 継続的 | 本番運用・継続改善 | ユーザーフィードバック・KPI監視 |

**総開発期間: 約8-12週間**

### 週次マイルストーン例 (Phase 1)
```
Week 1: JWT認証・基本API実装
Week 2: 音声管理API・通知基盤実装  
Week 3: セキュリティ強化・API統合テスト
```

### 週次マイルストーン例 (Phase 2) 
```
Week 1-2: Flutter プロジェクト初期化・認証実装
Week 3-4: メール機能・音声機能実装
Week 5-6: 設定機能・通知機能・統合テスト
```

---

## 🚀 実装成功のための重要指針

### 1. **段階的検証アプローチ**
```bash
# 各ステップでの動作確認必須
Phase 1: curl/Postman でAPI単体テスト
Phase 2: Flutter DevTools でアプリ動作確認
Phase 3: 実機・本番環境での結合テスト
Phase 4: ユーザー行動データでの継続検証
```

### 2. **コード品質維持**
```yaml
# 必須チェック項目
- [ ] 単体テストカバレッジ > 80%
- [ ] linter エラー 0件
- [ ] API レスポンス時間 < 500ms
- [ ] アプリ起動時間 < 3秒
- [ ] メモリ使用量 < 100MB
```

### 3. **技術的負債回避**
```go
// 実装時の注意点
- DRY原則の徹底 (重複コード排除)
- 適切なエラーハンドリング実装
- ログ出力の統一 (構造化ログ)
- 設定の外部化 (環境変数・設定ファイル)
- セキュリティ第一 (認証・認可の確実な実装)
```

### 4. **ユーザビリティ優先**
```dart
// モバイルUX のベストプラクティス
- Loading状態の適切な表示
- ネットワークエラー時の明確なメッセージ
- オフライン状態の分かりやすい表示
- アクセシビリティ対応 (VoiceOver/TalkBack)
- タッチ操作の最適化 (44pt以上のタップ領域)
```

### 5. **監視・改善サイクル**
```bash
# 継続的改善のための測定項目
- ユーザー行動ファネル分析
- パフォーマンス監視 (APM)
- エラー率・クラッシュ率監視
- A/Bテストによる機能改善
- ユーザーフィードバックの定期的収集・分析
```

---

## 🎯 プロジェクト完了条件

**MVP完了条件**:
- [ ] Google OAuth2ログインが安定動作
- [ ] Gmail API経由でメール一覧が取得できる
- [ ] OpenAI TTS で音声生成・再生ができる
- [ ] iOS/Android 両プラットフォームで動作
- [ ] App Store/Google Play にアプリが公開されている
- [ ] 基本的な設定・通知機能が動作している

**本ドキュメントにより、現在の高品質なGoバックエンドを最大限活用し、モバイルユーザーに最適化された体験を提供するアプリを、段階的かつ効率的に開発できます。** 