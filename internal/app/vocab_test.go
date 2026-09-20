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

func TestCheckVocabularyDevelopmentPatterns(t *testing.T) {
	content := `# 開発ドキュメントパターンテスト

別版へ黙って切り替えないこと。
別の Node へ黙って戻すことはありません。
リソースが無ければ作る方針です。
なければ作成する手順です。
Worker を実アカウントへ載せる。
probe Worker を載せる。
実 Cloudflare を叩くコマンドです。
応答がピンと違う場合は拒否する。
submodule の状態は汚れに数えない。
できた commit を取り込む。
テストを通したことにする。
各定義が対象判定とコマンドを持ちます。
warning も失敗にします。
`
	doc := mdparse.Parse([]byte(content))
	findings := checkVocabulary("dev.md", []byte(content), doc.Excluded)

	expectedSubstrings := []string{
		`awkward phrasing "黙って切り替え"`,
		`awkward phrasing "黙って戻す"`,
		`awkward phrasing "無ければ作る"`,
		`awkward phrasing "なければ作成する"`,
		`awkward phrasing "実アカウントへ載せる"`,
		`awkward phrasing "Worker を載せる"`,
		`awkward phrasing "Cloudflare を叩く"`,
		`awkward phrasing "ピンと違う"`,
		`awkward phrasing "汚れに数え"`,
		`awkward phrasing "できた commit"`,
		`awkward phrasing "通したことにする"`,
		`awkward phrasing "判定とコマンドを持ち"`,
		`awkward phrasing "warning も失敗に"`,
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

func TestCheckVocabularyADRPatterns(t *testing.T) {
	content := `# ADR パターンテスト

縮小か外出しか選べない状況です。
この障害への手当てがない状態です。
契約の消費者なしに進めることはできません。
すべてのタスクに直列着手するのは非効率です。
この構成では結合の検証が最も安い選択肢です。
リソースのライフサイクルは人が持つ運用とします。
`
	doc := mdparse.Parse([]byte(content))
	findings := checkVocabulary("adr.md", []byte(content), doc.Excluded)

	expectedSubstrings := []string{
		`awkward phrasing "外出しか"`,
		`awkward phrasing "手当てがない"`,
		`awkward phrasing "消費者なしに"`,
		`awkward phrasing "直列着手"`,
		`awkward phrasing "検証が最も安い"`,
		`awkward phrasing "ライフサイクルは人が持つ"`,
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

func TestCheckVocabularyIssuePatterns(t *testing.T) {
	content := `# 課題ドキュメントパターンテスト

後続の処理を塞ぐ恐れがあります。
エージェントへの接続を張れない状態です。
Workerの箱を用意する。
チームの所有物とする方針です。
システムが自分で閉じる動作です。
コードを触る単位を小さくする。
その掛け算をしていない試算です。
誰も回さないタスクが残る。
一覧が黄色になる状態を避ける。
逃げ道なしで失敗する。
死区間が発生してしまう。
自前の再配送を行う。
処理が走っていない状況です。
コピーの硬化を招く。
テストで達成する賭けに出る。
リストからだけ抜けている。
記述漏れを埋める。
自分の数値目標を掲げる。
書き分けが崩れている。
ドキュメントを正本にする。
問い自体が消えた。
本文に1文書く。
完了条件を置く。
仕様を理解する経路を設ける。
受ける側でしかない立場です。
外部へ黙って公開してはならない。
黙った移動を検知する。
`
	doc := mdparse.Parse([]byte(content))
	findings := checkVocabulary("issue.md", []byte(content), doc.Excluded)

	expectedSubstrings := []string{
		`awkward phrasing "後続の処理を塞ぐ"`,
		`awkward phrasing "エージェントへの接続を張れない"`,
		`awkward phrasing "Workerの箱"`,
		`awkward phrasing "の所有物とする"`,
		`awkward phrasing "システムが自分で閉じる"`,
		`awkward phrasing "コードを触る単位"`,
		`awkward phrasing "その掛け算をしていない"`,
		`awkward phrasing "誰も回さない"`,
		`awkward phrasing "一覧が黄色になる"`,
		`awkward phrasing "逃げ道なしで"`,
		`awkward phrasing "死区間"`,
		`awkward phrasing "自前の再配送"`,
		`awkward phrasing "処理が走っていない"`,
		`awkward phrasing "コピーの硬化"`,
		`awkward phrasing "で達成する賭け"`,
		`awkward phrasing "からだけ抜けている"`,
		`awkward phrasing "記述漏れを埋める"`,
		`awkward phrasing "自分の数値目標"`,
		`awkward phrasing "書き分けが崩れて"`,
		`awkward phrasing "正本にする" (consider "一次定義" or "信頼できる情報源（SSOT）"`,
		`awkward phrasing "問い自体が消えた"`,
		`awkward phrasing "本文に1文書く"`,
		`awkward phrasing "完了条件を置く"`,
		`awkward phrasing "理解する経路"`,
		`awkward phrasing "受ける側でしかない"`,
		`awkward phrasing "黙って公開"`,
		`awkward phrasing "黙った移動"`,
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

func TestCheckVocabularySlice2Patterns(t *testing.T) {
	content := `# スライス2パターンテスト

テスト結果が赤になった場合は停止する。
すべての検証を緑にする方針です。
緑 run の結果を確認する。
赤と緑の出し分けを検証する。
失敗したため撃ち直す。
2本目を撃つ前に確認する。
撃つ前に相談する。
結果が出るまで回すことは避ける。
プロセスが途中で死ぬことを防ぐ。
費用が丸損になる。
データを詰める側を実装する。
スタブとして形だけ置く。
binding なのは本数ではなく金額である。
状態を可視にする必要がある。
差分が人の目で分かるようにする。
読まれないと困る情報を配置する。
たぶん動くだろうという判断は避ける。
確認するに留める。
費用の正直な集計を残す。
`
	doc := mdparse.Parse([]byte(content))
	findings := checkVocabulary("slice2.md", []byte(content), doc.Excluded)

	expectedSubstrings := []string{
		`awkward phrasing "赤になった"`,
		`awkward phrasing "緑にする"`,
		`awkward phrasing "緑 run"`,
		`awkward phrasing "赤と緑の出し分け"`,
		`awkward phrasing "撃ち直す"`,
		`awkward phrasing "2本目を撃つ"`,
		`awkward phrasing "撃つ前"`,
		`awkward phrasing "出るまで回す"`,
		`awkward phrasing "途中で死ぬ"`,
		`awkward phrasing "丸損"`,
		`awkward phrasing "詰める側"`,
		`awkward phrasing "形だけ置く"`,
		`awkward phrasing "binding なのは"`,
		`awkward phrasing "可視にする"`,
		`awkward phrasing "人の目で分かる"`,
		`awkward phrasing "読まれないと困る"`,
		`awkward phrasing "たぶん動く"`,
		`awkward phrasing "に留める"`,
		`awkward phrasing "正直な集計"`,
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

func TestCheckVocabularySlice1Patterns(t *testing.T) {
	content := `# スライス1パターンテスト

Check が赤くならないことを確認する。
モデル出力の揺れを CIの赤にしない。
既存の test が緑であることを確かめる。
### 5.1 緑にするもの
### 5.2 通すもの
これ以上 controller を肥らせない。
index の step を割らない。
既存の browser test が落ちていないことを確認する。
不正な引数は実行前に落ちる。
戻り値をファイルへ落とす。
registered 枝を残さず落とす。
拒否時にセッションを殺さない。
エラー時に run を倒さない。
未知の値を failure へ倒す。
型違反に化けてしまう。
driver が状態を抱え込む。
発生したエラーを拾う。
勝手に判定を変更しない。
枠を広げない。
CLI から observe を撃つ。
fixture PR を起こす。
目で確認する。
破れないことを手で確かめる。
台帳が嘘をつく。
上限を超えても削って出さない。
版管理された JSON 1本で管理する。
既存のテストを1本作成する。
コードを足す。
担当者一覧を足した構成。
不正な値はバリデーションで跳ねる。
丸ごとファイルへ保存する。
完了するまで見る。
`
	doc := mdparse.Parse([]byte(content))
	findings := checkVocabulary("slice1.md", []byte(content), doc.Excluded)

	expectedSubstrings := []string{
		`awkward phrasing "赤くなら"`,
		`awkward phrasing "CIの赤"`,
		`awkward phrasing "test が緑"`,
		`awkward phrasing "緑にするもの"`,
		`awkward phrasing "通すもの"`,
		`awkward phrasing "肥らせな"`,
		`awkward phrasing "step を割ら"`,
		`awkward phrasing "test が落ちて"`,
		`awkward phrasing "実行前に落ちる"`,
		`awkward phrasing "ファイルへ落とす"`,
		`awkward phrasing "枝を残さず落とす"`,
		`awkward phrasing "セッションを殺さ"`,
		`awkward phrasing "run を倒さ"`,
		`awkward phrasing "failure へ倒す"`,
		`awkward phrasing "型違反に化けて"`,
		`awkward phrasing "状態を抱え込"`,
		`awkward phrasing "エラーを拾う"`,
		`awkward phrasing "勝手に判定"`,
		`awkward phrasing "枠を広げな"`,
		`awkward phrasing "observe を撃つ"`,
		`awkward phrasing "PR を起こす"`,
		`awkward phrasing "目で確認"`,
		`awkward phrasing "手で確かめ"`,
		`awkward phrasing "台帳が嘘をつく"`,
		`awkward phrasing "削って出さ"`,
		`awkward phrasing "JSON 1本"`,
		`awkward phrasing "テストを1本"`,
		`awkward phrasing "コードを足す"`,
		`awkward phrasing "一覧を足し"`,
		`awkward phrasing "バリデーションで跳ねる"`,
		`awkward phrasing "丸ごとファイル"`,
		`awkward phrasing "完了するまで見る"`,
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

