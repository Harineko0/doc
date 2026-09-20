package app

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"unicode/utf8"
)

type vocabRule struct {
	pattern    *regexp.Regexp
	suggestion string
}

var vocabRules = []vocabRule{
	// Budget and quota slang
	{
		pattern:    regexp.MustCompile(`財布`),
		suggestion: `"利用枠" or "予算枠"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:予約|枠|予算|クォータ)[^。\n]{0,15}借用`),
		suggestion: `"流用" or "前借り"`,
	},

	// Test result slang
	{
		pattern:    regexp.MustCompile(`赤に(?:する|しない|なら|せ)`),
		suggestion: `"失敗とする" or "失敗として扱わない"`,
	},
	{
		pattern:    regexp.MustCompile(`赤(?:が無い|がない|となる|として)`),
		suggestion: `"失敗" or "テスト失敗"`,
	},
	{
		pattern:    regexp.MustCompile(`判定の赤`),
		suggestion: `"テスト失敗の判定"`,
	},

	// Colloquial / slang metaphors
	{
		pattern:    regexp.MustCompile(`太らせ[るりれ]`),
		suggestion: `"機能を拡充する" or "拡張する"`,
	},
	{
		pattern:    regexp.MustCompile(`幅を削[るりれ]`),
		suggestion: `"対象範囲を絞り込む" or "スコープを縮小する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:に|へ)割れて(?:いる|お|い)`),
		suggestion: `"分割されている"`,
	},
	{
		pattern:    regexp.MustCompile(`辺を(?:1本)?(?:増やす|増やし|作る|作り)`),
		suggestion: `"連携経路を増やす" or "接続を追加する"`,
	},
	{
		pattern:    regexp.MustCompile(`辺の(?:追加|作成)`),
		suggestion: `"連携経路の追加" or "接続の作成"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:上へ)?舐め[るたて]`),
		suggestion: `"網羅的に走査する" or "順次試行する"`,
	},
	{
		pattern:    regexp.MustCompile(`手足を?作る`),
		suggestion: `"実行基盤を構築する"`,
	},
	{
		pattern:    regexp.MustCompile(`手足となる`),
		suggestion: `"実行基盤となる"`,
	},
	{
		pattern:    regexp.MustCompile(`脳に紐づ`),
		suggestion: `"モデル構成に依存する" or "モデル構成に紐づく"`,
	},
	{
		pattern:    regexp.MustCompile(`黙って(?:切断|切り詰め|フォールバック|置き換え|落と|終了|無視)`),
		suggestion: `"通知なく" or "暗黙に"`,
	},
	{
		pattern:    regexp.MustCompile(`成功を作る`),
		suggestion: `"成功状態を作り出す" or "成功と偽装する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:unknown|エラー|失敗)(?:に|へ)?落とす`),
		suggestion: `"状態に遷移させる"`,
	},
	{
		pattern:    regexp.MustCompile(`回帰(?:テスト)?を落とす`),
		suggestion: `"回帰テストを失敗させる"`,
	},

	// English "has" calques
	{
		pattern:    regexp.MustCompile(`順序を持つ`),
		suggestion: `"順序を定義する" or "順序を定める"`,
	},
	{
		pattern:    regexp.MustCompile(`進捗を持つ`),
		suggestion: `"進捗を管理する"`,
	},
	{
		pattern:    regexp.MustCompile(`責務として持つ`),
		suggestion: `"責務として定義する" or "責務を担う"`,
	},
	{
		pattern:    regexp.MustCompile(`前提(?:事項)?(?:と[^。\n]{0,10})?を持つ`),
		suggestion: `"前提を定義する" or "前提をまとめる"`,
	},
	{
		pattern:    regexp.MustCompile(`中身は持たない`),
		suggestion: `"内容は扱わない" or "詳細を含まない"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:予算|財布)[^。\n]{0,10}を持つ`),
		suggestion: `"予算枠を管理する"`,
	},

	// Other awkward phrasing
	{
		pattern:    regexp.MustCompile(`先に当たった方`),
		suggestion: `"先に達した方" or "先に条件を満たした方"`,
	},
	{
		pattern:    regexp.MustCompile(`積む量`),
		suggestion: `"割り当てる量" or "設定量"`,
	},
	{
		pattern:    regexp.MustCompile(`合わせに行か(?:ない|ず)`),
		suggestion: `"過剰に適応しない" or "無理に追従しない"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:後ろ|後)へ?に回す`),
		suggestion: `"後続工程に送る" or "後回しにする"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:確認|検証)を後段(?:や他レーン)?へ送ら`),
		suggestion: `"検証を後続工程に先送りしない"`,
	},
}

type rawMatch struct {
	start      int
	end        int
	suggestion string
}

func checkVocabulary(path string, data []byte, excluded []bool) []finding {
	var matches []rawMatch
	for _, rule := range vocabRules {
		locs := rule.pattern.FindAllIndex(data, -1)
		for _, loc := range locs {
			start, end := loc[0], loc[1]
			// Skip matches inside code blocks, code spans, or raw HTML.
			if isExcluded(excluded, start, end) {
				continue
			}
			matches = append(matches, rawMatch{
				start:      start,
				end:        end,
				suggestion: rule.suggestion,
			})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort matches by start position, preferring longer matches on ties.
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].start != matches[j].start {
			return matches[i].start < matches[j].start
		}
		return matches[i].end > matches[j].end
	})

	// Filter out overlapping matches.
	var nonOverlapping []rawMatch
	lastEnd := -1
	for _, m := range matches {
		if m.start < lastEnd {
			continue
		}
		nonOverlapping = append(nonOverlapping, m)
		lastEnd = m.end
	}

	findings := make([]finding, 0, len(nonOverlapping))
	for _, m := range nonOverlapping {
		line := 1 + bytes.Count(data[:m.start], []byte{'\n'})
		lastNL := bytes.LastIndexByte(data[:m.start], '\n')
		column := utf8.RuneCount(data[lastNL+1:m.start]) + 1
		phrase := string(data[m.start:m.end])
		msg := fmt.Sprintf("awkward phrasing %q (consider %s)", phrase, m.suggestion)
		findings = append(findings, finding{
			path:    path,
			line:    line,
			column:  column,
			message: msg,
		})
	}
	return findings
}

func isExcluded(excluded []bool, start, end int) bool {
	if len(excluded) == 0 {
		return false
	}
	limit := min(end, len(excluded))
	for i := start; i < limit; i++ {
		if excluded[i] {
			return true
		}
	}
	return false
}
