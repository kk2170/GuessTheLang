# Guess The Lang

[![CI](https://github.com/kk2170/GuessTheLang/actions/workflows/ci.yml/badge.svg)](https://github.com/kk2170/GuessTheLang/actions/workflows/ci.yml)

Yes / No で答えていく、**プログラミング言語アキネーター**です。  
Go 製のシングルバイナリ CLI で、**外した言語を次回から学習**できます。

## 特徴

- Go 製
- **標準ライブラリのみ**で実装
- `go build` で **ワンバイナリ**になる
- 候補を一番よく分けられる質問を毎回選ぶ
- 完全一致がなければ近い候補を返す
- 外したときに **正解の言語を学習して保存**できる
- 収録言語を初期状態から増やしてある

## 収録言語

Go, Rust, C, C++, Java, C#, JavaScript, TypeScript, Python, Ruby, PHP, Swift, Kotlin, Haskell, Scala, Elixir, Dart, Bash, PowerShell, Perl, Lua, Objective-C, Clojure, F#, R, Julia, Nim, Fortran, COBOL, Ada, OCaml, Erlang, Common Lisp, Scheme, Visual Basic .NET, MATLAB

## 実行

```bash
go run .
```

## ビルド

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o guess-the-lang .
```

生成された `guess-the-lang` だけで動かせます。

## 遊び方

- `y` / `yes` / `はい` : Yes
- `n` / `no` / `いいえ` : No
- `?` / `skip` / `わからない` : 分からないのでスキップ

言語を当てられなかったときや、予想が外れたときは、正解の言語名と特徴を教えることで次回から候補に追加されます。

## 学習データの保存先

デフォルトでは次のファイルに保存します。

```text
~/.config/guess-the-lang/knowledge.json
```

保存先を変えたい場合は環境変数で上書きできます。

```bash
GUESS_THE_LANG_DATA=/path/to/knowledge.json ./guess-the-lang
```

## サンプル

```text
Guess The Lang
Yes / No で答えてください。分からない場合は ? でスキップできます。

- Web バックエンドでよく使われますか？ [y/n/?]: y
- C 系の波括弧構文ですか？ [y/n/?]: y
- ブラウザでそのまま動くことが多いですか？ [y/n/?]: n
- 主にネイティブコードへコンパイルして使いますか？ [y/n/?]: y

たぶん Go です。
根拠: ネイティブコンパイル / Web バックエンド / C 系の波括弧構文
ひとこと: シンプルな文法と goroutine による軽量並行処理が特徴です。
```

## 開発

```bash
go test ./...
```

GitHub Actions でも push / pull request ごとにテストとビルドを実行します。

## 拡張方法

- `features` に質問を追加する
- `builtinLanguages` に初期言語を追加する
- 学習済みデータを `knowledge.json` に蓄積する
