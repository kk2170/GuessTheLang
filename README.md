# Guess The Lang

[![CI](https://github.com/kk2170/GuessTheLang/actions/workflows/ci.yml/badge.svg)](https://github.com/kk2170/GuessTheLang/actions/workflows/ci.yml)

Yes / No で答えていく、**プログラミング言語アキネーター**です。  
Go 製のシングルバイナリ CLI で、**外した言語を次回から学習**できます。

## 特徴

- Go 製
- **標準ライブラリのみ**で実装
- `go build` で **ワンバイナリ**になる
- 内部は `Catalog` / `Questions` / `Entries` のデータ構造で動く
- デフォルトの言語 catalog も `catalogs/programming-languages.json` を埋め込んで使う
- 候補を一番よく分けられる質問を毎回選ぶ（同点なら少しランダム）
- 完全一致がなければ近い候補を返す
- 外したときに **正解の言語を学習して保存**できる
- 収録言語を初期状態から増やしてある

デフォルトでは「プログラミング言語カタログ」を使っていますが、同じ推論ロジックを別テーマの `Catalog` に差し替えて再利用できます。

## 収録言語

Go, Rust, C, C++, Java, C#, JavaScript, TypeScript, Python, Ruby, PHP, Swift, Kotlin, Haskell, Scala, Elixir, Dart, Bash, PowerShell, Perl, Lua, BASIC, Objective-C, Clojure, F#, R, Julia, Nim, Fortran, COBOL, Ada, ABAP, ALGOL, APL, AppleScript, AutoIt, AWK, Ballerina, BCPL, Carbon, Chapel, CoffeeScript, D, Delphi, Eiffel, Elm, Emacs Lisp, Forth, Groovy, Hack, Haxe, J, ksh, LabVIEW, LOGO, Modula-2, OCaml, Oberon, Oz, Pascal, Processing, Prolog, PostScript, Racket, REBOL, SAS, Scratch, Self, Simula, Smalltalk, SNOBOL, Standard ML, Tcl, Visual FoxPro, Erlang, Common Lisp, Scheme, Visual Basic .NET, MATLAB

## 実行

```bash
go run .
```

外部 catalog を使いたい場合は、JSON ファイルを指定できます。

```bash
go run . -catalog /path/to/catalog.json
```

または環境変数でも指定できます。

```bash
GUESS_THE_LANG_CATALOG=/path/to/catalog.json go run .
```

リポジトリ内の sample catalog を試す例:

```bash
go run . -catalog catalogs/animals.json
go run . -catalog catalogs/fruits.json
```

### catalog コマンド

雛形を作る:

```bash
go run . catalog init ./my-catalog.json
go run . catalog init --id animals --title "Guess The Animal" ./animals.json
```

妥当性を検証する:

```bash
go run . catalog validate ./my-catalog.json
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

外部 catalog を使う場合、`GUESS_THE_LANG_DATA` を指定しなければ catalog ごとに別の学習ファイルを使います。

同じ catalog を別パスから開いても同じ学習履歴を使いたい場合は、catalog JSON に `id` を入れてください。

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

- `defaultCatalog.Questions` に質問を追加する
- `defaultCatalog.Entries` に初期候補を追加する
- 学習済みデータを `knowledge.json` に蓄積する
- 別テーマにしたい場合は `Catalog{Title, Intro, Questions, Entries}` を差し替える

### catalog JSON 例

```json
{
  "id": "animals",
  "title": "Guess The Animal",
  "intro": "動物について yes / no で答えてください。",
  "questions": [
    { "key": "barks", "label": "吠える", "prompt": "吠えますか？" },
    { "key": "meows", "label": "鳴く", "prompt": "よく鳴きますか？" }
  ],
  "entries": [
    { "name": "Dog", "summary": "Barks loudly.", "features": ["barks"] },
    { "name": "Cat", "summary": "Often meows.", "features": ["meows"] }
  ]
}
```

### catalog schema

- `id` : 任意。catalog を安定識別する文字列。別パスから開いても同じ学習履歴を使いたいなら設定します。
- `title` : 必須。ゲーム開始時のタイトルです。
- `intro` : 必須。開始時の説明文です。
- `questions` : 必須。1 件以上必要です。
  - `key` : 必須。内部識別子。
  - `label` : 必須。根拠表示などで使う短い名前。
  - `prompt` : 必須。実際に表示する質問文。
- `entries` : 必須。1 件以上必要です。
  - `name` : 必須。候補名。
  - `summary` : 任意に近い説明文ですが、表示上ほぼ必須です。
  - `features` : その候補に当てはまる `question.key` の配列。

制約:

- `question.key` は重複不可
- `entry.name` は重複不可
- `entry.features` は必ず既存の `question.key` を参照する必要があります
- `id` は空でなく、少なくとも 1 つは英数字を含む必要があります
