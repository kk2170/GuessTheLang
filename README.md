# Guess The Lang

Yes / No で答えていく、**プログラミング言語アキネーター**です。

## 特徴

- Go 製
- **標準ライブラリのみ**で実装
- `go build` で **ワンバイナリ**になる
- 候補を一番よく分けられる質問を毎回選ぶ
- 完全一致がなければ近い候補を返す

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

## 収録言語

Go, Rust, C, C++, Java, C#, JavaScript, TypeScript, Python, Ruby, PHP, Swift, Kotlin, Haskell, Scala, Elixir, Dart

## 拡張方法

`main.go` の `features` と `languages` を増やせば、そのまま質問精度を上げられます。
