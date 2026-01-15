# lazyccg

Claude Code / Codex / Gemini のセッション状態を TUI で可視化する軽量ダッシュボード。

## 目的
- 複数セッションの状態を一覧で把握
- 入力待ちや完了を即座に判別
- lazydocker 風の分割ペイン UI

## 対象
- Claude Code
- Codex
- Gemini

## MVP (最小機能)
- セッション一覧表示 (状態, AI種別)
- 選択セッションのイベントログ表示
- 入力待ち状態の強調表示 (推定)
- キーボード操作のみ (上下選択, Enter で該当タブに移動, q で終了)

## 前提
- Kitty のリモートコントロールを使う
- タブタイトルは `codex:xxx` / `claude:xxx` / `gemini:xxx` の形式

例:
```bash
kitty @ set-tab-title "codex:projA"
```

## 使い方 (予定)
```bash
cd lazyccg
# 初回のみ
# go mod tidy

# 起動
go run ./cmd/lazyccg
```

## オプション
- `-poll` 更新間隔 (例: 1s)
- `-prefixes` 対象AIの接頭辞 (例: codex,claude,gemini)
- `-max-lines` 取得する最大行数

## 画面イメージ
- 左: セッション一覧
- 右: 選択セッションの出力
- 下: ステータスバー

## 次に決めること
- ステータス推定の精度向上
- 取得範囲の最適化 (表示行数, 取得頻度)
- Kitty以外への拡張
