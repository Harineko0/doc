package app

import (
	"strings"
	"testing"

	"github.com/Harineko0/doc/internal/mdparse"
)

func TestCheckVocabulary(t *testing.T) {
	content := `# サンプルドキュメント

これは実モデルの財布を消費する処理です。
また、機能を壊さずに太らせることが目的です。
この設計は深さを削るのではなく、幅を削る方針です。
テスト結果を赤にする判定は避けてください。
スライスの進捗を持つ台帳を作成します。
前提事項を持つ必要があります。
実装の順序を持つ必要があります。
`
	doc := mdparse.Parse([]byte(content))
	findings := checkVocabulary("test.md", []byte(content), doc.Excluded)

	if len(findings) == 0 {
		t.Fatal("expected vocabulary findings, got none")
	}

	expectedSubstrings := []string{
		`awkward phrasing "財布"`,
		`awkward phrasing "太らせる"`,
		`awkward phrasing "幅を削る"`,
		`awkward phrasing "赤にする"`,
		`awkward phrasing "進捗を持つ"`,
		`awkward phrasing "順序を持つ"`,
	}

	var joined strings.Builder
	for _, f := range findings {
		joined.WriteString(f.String())
		joined.WriteByte('\n')
	}
	output := joined.String()

	for _, expected := range expectedSubstrings {
		if !strings.Contains(output, expected) {
			t.Errorf("missing expected diagnostic %q in output:\n%s", expected, output)
		}
	}
}

func TestCheckVocabularyIgnoresCodeBlocksAndSpans(t *testing.T) {
	content := `# コード除外テスト

プログラミング用語としての ` + "`財布`" + ` や ` + "`太らせる`" + ` の言及。

` + "```yaml" + `
description: "財布を分ける設定"
action: "壊さずに太らせる"
` + "```" + `

    インデントされたコードブロック内の財布や赤にする判定

正常な記述：利用枠を適切に管理し、機能を拡充します。
`
	doc := mdparse.Parse([]byte(content))
	findings := checkVocabulary("code.md", []byte(content), doc.Excluded)

	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for code blocks/spans, got %d: %v", len(findings), findings)
	}
}

func TestCheckVocabularyColumnAndLine(t *testing.T) {
	// Line 1: Header (23 chars)
	// Line 2: Empty
	// Line 3: "これは" (3 runes) + "財布" at rune 4 (1-based)
	content := `# テストタイトル

これは財布です。
`
	doc := mdparse.Parse([]byte(content))
	findings := checkVocabulary("doc.md", []byte(content), doc.Excluded)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.line != 3 {
		t.Errorf("expected line 3, got %d", f.line)
	}
	if f.column != 4 {
		t.Errorf("expected column 4, got %d", f.column)
	}
}

func TestLintCatchesAwkwardVocabularyInWorkingTree(t *testing.T) {
	repo := newRepo(t)
	write(t, repo, "README.md", "# ドキュメント\n\n実モデルの財布を管理する。\n")

	withCWD(t, repo, func() {
		findings, err := Lint(false)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0], `awkward phrasing "財布"`) {
			t.Errorf("unexpected finding: %s", findings[0])
		}
	})
}

func TestCheckVocabularyAdditionalPatterns(t *testing.T) {
	content := `# 追加パターンテスト

6-A と 6-B に割れている構成です。
実環境の辺を1本増やす必要があります。
手足を作る段階で設計します。
現行の脳に紐づく仕様です。
エラー時に黙って切断しないでください。
ケースの失敗時に回帰を落とす動作になります。
このファイルは中身は持たない方針です。
先に当たった方で処理を停止します。
`
	doc := mdparse.Parse([]byte(content))
	findings := checkVocabulary("additional.md", []byte(content), doc.Excluded)

	expectedSubstrings := []string{
		`awkward phrasing "に割れている"`,
		`awkward phrasing "辺を1本増やす"`,
		`awkward phrasing "手足を作る"`,
		`awkward phrasing "脳に紐づ"`,
		`awkward phrasing "黙って切断"`,
		`awkward phrasing "回帰を落とす"`,
		`awkward phrasing "中身は持たない"`,
		`awkward phrasing "先に当たった方"`,
	}

	var joined strings.Builder
	for _, f := range findings {
		joined.WriteString(f.String())
		joined.WriteByte('\n')
	}
	output := joined.String()

	for _, expected := range expectedSubstrings {
		if !strings.Contains(output, expected) {
			t.Errorf("missing expected diagnostic %q in output:\n%s", expected, output)
		}
	}
}
