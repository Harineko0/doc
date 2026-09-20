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
		pattern:    regexp.MustCompile(`赤に(?:する|しない|なら|せ|なった|なっ)`),
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
	{
		pattern:    regexp.MustCompile(`緑に(?:する|したもの|して)`),
		suggestion: `"パスさせる" or "成功条件とする"`,
	},
	{
		pattern:    regexp.MustCompile(`緑\s?run|緑を得[るた]`),
		suggestion: `"成功した run" or "成功結果を得る"`,
	},
	{
		pattern:    regexp.MustCompile(`赤と緑(?:を|の)出し分け`),
		suggestion: `"合否の検証" or "成功と失敗の検証"`,
	},
	{
		pattern:    regexp.MustCompile(`赤くな[るりっら]|赤く(?:する|しない)`),
		suggestion: `"失敗する" or "エラー表示となる"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:CI|テスト|判定|Check|e2e)の赤`),
		suggestion: `"テスト失敗" or "失敗状態"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:CI|test|テスト|build|Checks?|suite|ジョブ)[^。\n]{0,10}(?:が|も)緑|全部緑|緑であること|緑で終わる`),
		suggestion: `"パスする" or "成功する"`,
	},
	{
		pattern:    regexp.MustCompile(`緑にするもの`),
		suggestion: `"成功させるべきテスト" or "パス条件"`,
	},
	{
		pattern:    regexp.MustCompile(`通すもの`),
		suggestion: `"満たすべき完了条件" or "合格条件"`,
	},

	// Colloquial / slang metaphors
	{
		pattern:    regexp.MustCompile(`(?:太|肥)らせ[るりれな]`),
		suggestion: `"機能を拡充する" or "肥大化を防ぐ"`,
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
		pattern:    regexp.MustCompile(`黙って(?:切断|切り詰め|切り替[ええ]?|フォールバック|置き換え|落と|終了|無視|戻[すし]?|公開)|黙った(?:移動|変更)`),
		suggestion: `"通知なく" or "暗黙に"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:無|な)ければ(?:作[るりれ]|作成す[るれ]|作成し)`),
		suggestion: `"存在しなければ作成する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:実?アカウント|環境)(?:へ|に)載せ[るたて]?|(?:Worker|probe|サービス|コンテナ)\s?を載せ[るたて]?`),
		suggestion: `"デプロイする" or "配置する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:Cloudflare|API|エンドポイント|サーバー)\s?を叩[くきいて]`),
		suggestion: `"リクエストを送信する" or "呼び出す" or "テストを実行する"`,
	},
	{
		pattern:    regexp.MustCompile(`ピンと(?:違[ういえ]|一致)`),
		suggestion: `"固定バージョンと異なる" or "固定値と一致しない"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:作業ツリーの)?汚れに数え|ツリーが汚[れれ]`),
		suggestion: `"変更（dirty）" or "未コミットの変更"`,
	},
	{
		pattern:    regexp.MustCompile(`できた\s?(?:コミット|commit)`),
		suggestion: `"生成されたコミット" or "作成されたコミット"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:通し?|成功し?|パスし?|合格し?)たことにす[るれ]`),
		suggestion: `"合格とみなす" or "パスしたとみなす"`,
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
	{
		pattern:    regexp.MustCompile(`(?:テスト|test|suite|ジョブ)[^。\n]{0,10}が落ち[るてた]|落ちて(?:いない|おら)`),
		suggestion: `"テストが失敗する" or "テストが失敗していない"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:実行前|事前|ゲート|検証)[^。\n]{0,10}(?:で|に)落ち[るてた]`),
		suggestion: `"エラー終了する" or "拒否される"`,
	},
	{
		pattern:    regexp.MustCompile(`ファイル(?:へ|に)落と[すしせ]`),
		suggestion: `"ファイルに出力する" or "ファイルに書き出す"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:残さず|schema|枝|プロパティ|定義)[^。\n]{0,10}(?:から)?落と[すしせ]`),
		suggestion: `"除外する" or "削除する"`,
	},
	{
		pattern:    regexp.MustCompile(`が買(?:うのは|ったのは)`),
		suggestion: `"得られるのは" or "もたらすのは"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:スライス|タスク|単位|step|ステップ|PR|run|コード)[^。\n]{0,15}(?:に|を)割[るりれら]|(?:[0-9０-９一二三四五六七八九十]+(?:つ|本))に割[るりれ]`),
		suggestion: `"分割する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:形|案|方式)へ(?:倒す|倒し|倒せ)`),
		suggestion: `"方式に切り替える" or "方針を採用する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:session|セッション|プロセス|コンテナ|sandbox|Worker|ブラウザ|接続)[^。\n]{0,10}を殺[すせしたさ]`),
		suggestion: `"終了させる" or "破棄する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:run|ジョブ)[^。\n]{0,10}を倒[すしたせさ]`),
		suggestion: `"異常終了させる" or "異常終了させない"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:partial|failure|[a-zA-Z_]+|失敗|異常)\s*(?:へ|に)?\s*倒(?:[れす][るたれ]|す|し|せ|さ)`),
		suggestion: `"状態に遷移させる" or "フォールバックする"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:上限|限界|枠)に張り付[くきいて]`),
		suggestion: `"上限に達する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:インスタンス|instance)に取り付[くきいて]`),
		suggestion: `"接続する" or "アタッチする"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:画像|DOM|Observation|文脈|コンテキスト)[^。\n]{0,20}を剥が[すしせ]`),
		suggestion: `"除去する" or "間引く"`,
	},
	{
		pattern:    regexp.MustCompile(`本物の依存`),
		suggestion: `"直接の依存関係"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:後続|処理|run|キュー)[^。\n]{0,10}を塞[ぐぎげ]`),
		suggestion: `"ブロックする" or "阻害する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:エージェント|接続|認証)[^。\n]{0,15}張れ(?:ない|ず)`),
		suggestion: `"確立できない" or "検証できない"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:gateway|ゲートウェイ|Worker|API)の箱`),
		suggestion: `"受け皿" or "枠組み"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:の)?所有物と(?:する|した|して)`),
		suggestion: `"所掌とする" or "担当範囲とする"`,
	},
	{
		pattern:    regexp.MustCompile(`システムが自分で閉じる`),
		suggestion: `"自動で完了させる" or "正常終了させる"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:コード|state|ステート|機能|テーブル|基盤)を触る(?:単位|とき|際)`),
		suggestion: `"改修する単位" or "改修する際"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:この|その)?掛け算をして(?:いな[いく]|おら(?:ず|ない)?)`),
		suggestion: `"試算していない" or "考慮していない"`,
	},
	{
		pattern:    regexp.MustCompile(`誰も回さ(?:ない|ず)`),
		suggestion: `"定期実行されない" or "誰も実行しない"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:一覧|画面)が(?:全部|すべて)?黄色にな[るりっ]`),
		suggestion: `"警告で埋め尽くされ" or "形骸化し"`,
	},
	{
		pattern:    regexp.MustCompile(`逃げ道なしで`),
		suggestion: `"代替手段がなく"`,
	},
	{
		pattern:    regexp.MustCompile(`死区間`),
		suggestion: `"停滞期間" or "デッドタイム"`,
	},
	{
		pattern:    regexp.MustCompile(`自前(?:の|で)再配送`),
		suggestion: `"独自の再配送"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:run|ジョブ|処理)が(?:1回も)?走って(?:いない|おら)`),
		suggestion: `"実行されていない"`,
	},
	{
		pattern:    regexp.MustCompile(`コピーの硬化`),
		suggestion: `"堅牢化（過剰な保護）"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:で達成する|による)賭け`),
		suggestion: `"不確実性" or "懸念"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:エラー|例外|コード|型|値)[^。\n]{0,10}(?:に|へと)化け[てるた]`),
		suggestion: `"変質する" or "意図しない型に変換される"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:状態|state|データ|メモリ)[^。\n]{0,10}を抱え[る込]`),
		suggestion: `"保持する" or "管理する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:エラー|例外|イベント|メッセージ|変更)[^。\n]{0,10}を拾[うったてい]`),
		suggestion: `"捕捉する" or "検知する" or "受信する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:スコープ|範囲|権限|トラップ|trap|制限|枠)[^。\n]{0,10}を広げ[るたてない]`),
		suggestion: `"拡張する" or "拡大する" or "緩和する"`,
	},

	// Execution and run slang
	{
		pattern:    regexp.MustCompile(`撃ち直[すし]`),
		suggestion: `"再実行する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:[0-9０-９一二三四五六七八九十]+\s*)?(?:本|回|run|リクエスト|API|observe|コマンド|手[で動]|CLI)[^。\n]{0,10}撃[つちたて]|撃つ前|撃って(?:は|も|お|い)|撃った(?:コマンド|処理|操作)`),
		suggestion: `"実行する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:run|ジョブ|PR|イベント|障害)[^。\n]{0,10}を起こ[すし]|意図的に起こ[すし]`),
		suggestion: `"開始する" or "トリガーする" or "再現させる"`,
	},
	{
		pattern:    regexp.MustCompile(`出るまで[^。\n]{0,10}(?:回す|書き換|再実行)`),
		suggestion: `"試行錯誤的に繰り返す" or "結果が出るまで再試行する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:が|で|時に|途中で|全体が)\s?死[ぬんだ]|死んで(?:いる|お|い)`),
		suggestion: `"異常終了する" or "機能していない"`,
	},
	{
		pattern:    regexp.MustCompile(`丸損`),
		suggestion: `"無駄になる" or "全損"`,
	},
	{
		pattern:    regexp.MustCompile(`詰める側`),
		suggestion: `"設定する側" or "構築する側"`,
	},
	{
		pattern:    regexp.MustCompile(`形だけ(?:置く|作[るり]|決め[るて])`),
		suggestion: `"スタブのみ作成する" or "枠組みのみ定義する"`,
	},

	// English "has" and loanword calques
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
	{
		pattern:    regexp.MustCompile(`(?:判定|コマンド)(?:と[^。\n]{0,10})?を持[つち]`),
		suggestion: `"定義する" or "備える"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:ライフサイクル|作成|所有|管理)は人が持[つち]`),
		suggestion: `"人間（運用者）が管理する" or "人が担当する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:binding|Binding)\s?(?:は|なのは|なのも)`),
		suggestion: `"拘束力を持つのは" or "制約となるのは"`,
	},


	// Other awkward phrasing
	{
		pattern:    regexp.MustCompile(`(?:warning|警告)\s?も失敗に`),
		suggestion: `"警告もエラーとして扱う" or "警告でも失敗とする"`,
	},
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
	{
		pattern:    regexp.MustCompile(`(?:から)?だけ抜けて(?:いる|お|い)|だけ落ちて(?:いる|お|い)`),
		suggestion: `"抜け落ちている" or "欠落している"`,
	},
	{
		pattern:    regexp.MustCompile(`記述漏れを埋め[るて]?`),
		suggestion: `"記述漏れを補正する" or "追記する"`,
	},
	{
		pattern:    regexp.MustCompile(`自分の数値目標`),
		suggestion: `"設定した数値目標"`,
	},
	{
		pattern:    regexp.MustCompile(`書き分けが崩れて`),
		suggestion: `"境界が曖昧になっている"`,
	},
	{
		pattern:    regexp.MustCompile(`正本(?:にする|とする|として|の置き場|であ[るり]|ではない)?`),
		suggestion: `"一次定義" or "信頼できる情報源（SSOT）" or "マスター" or "正とする"`,
	},
	{
		pattern:    regexp.MustCompile(`問い(?:自体)?が消え(?:た|る)`),
		suggestion: `"課題自体が解消した"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:本文に)?(?:この方針を)?1文書[くきいた]`),
		suggestion: `"1文明記する" or "1文記載する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:着手条件|完了条件|前提条件)を置く`),
		suggestion: `"定義する" or "管理する" or "記載する"`,
	},
	{
		pattern:    regexp.MustCompile(`理解する経路`),
		suggestion: `"把握する経路" or "把握する仕組み"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:受ける|利用する)側でしかない`),
		suggestion: `"側にすぎない"`,
	},
	{
		pattern:    regexp.MustCompile(`外出しか`),
		suggestion: `"スコープ外への切り出し" or "対象外への移行"`,
	},
	{
		pattern:    regexp.MustCompile(`手当て(?:が(?:な[いく]|無[いく]?)|の置き場)`),
		suggestion: `"対策" or "対応策"`,
	},
	{
		pattern:    regexp.MustCompile(`消費者(?:なしに|のない|がある|を作る|ができた)`),
		suggestion: `"利用側" or "利用箇所"`,
	},
	{
		pattern:    regexp.MustCompile(`直列着手`),
		suggestion: `"直列的な着手" or "順次着手"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:検証|テスト|確認|実装)[^。\n]{0,15}が最も安[くい]`),
		suggestion: `"最も低コスト" or "最もコストが小さい"`,
	},
	{
		pattern:    regexp.MustCompile(`可視に(?:する|して|した|しない)`),
		suggestion: `"可視化する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:人の)?目で(?:確認|確かめ|見[るてた]|追[ういて]|分か[るり]|わかる)`),
		suggestion: `"目視で確認する" or "目視で判別する"`,
	},
	{
		pattern:    regexp.MustCompile(`手で(?:確かめ|撃[つちたて]|起こ[すし]|動か[すし])`),
		suggestion: `"手動で確認する" or "手動で実行する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:台帳|PR|記録|状態|値|コメント)[^。\n]{0,10}(?:が|に)?嘘(?:をつ[くきいた]|が|の|に|載[るりっ])|嘘をつ[くきいた]`),
		suggestion: `"事実と乖離する" or "事実と異なる内容が掲載される"`,
	},
	{
		pattern:    regexp.MustCompile(`削って(?:出さ|投稿|送ら|表示)`),
		suggestion: `"切り詰めて投稿しない" or "省略して送信しない"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:テスト|test|JSON|ルート|route|セッション|session|実証|クエリ|query|PR|ブランチ|エンドポイント|API|更新|コマンド|run)\s*(?:が|を|で|の|は)?\s*[0-9０-９一二三四五六七八九十]+\s*本(?:も)?|[0-9０-９一二三四五六七八九十]+\s*本(?:目)?の(?:テスト|test|JSON|ルート|route|セッション|session|実証|クエリ|query|PR|ブランチ|エンドポイント|API|更新|コマンド|run)`),
		suggestion: `"1件" or "1つの" or "単一の"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:機能|コード|テスト|test|API|ルート|route|テーブル|カラム|列|一覧|画面|パラメータ|プロパティ|項目|単位)[^。\n]{0,10}を足[すしたて]|足した(?:こと|もの|結果)|足す(?:機能|コード|テスト|test|API|ルート|route|テーブル|カラム|画面|内容)`),
		suggestion: `"追加する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:バリデーション|検証|判定|検査|ガード|gate)[^。\n]{0,10}(?:で|に)?跳ね[るられ]`),
		suggestion: `"拒否される" or "エラーとして弾く"`,
	},
	{
		pattern:    regexp.MustCompile(`丸ごと(?:ファイル|出力|保存|コピー|送信|ダンプ)`),
		suggestion: `"すべて" or "そのまま"`,
	},
	{
		pattern:    regexp.MustCompile(`勝手に(?:枠|スコープ|判定|変更|判断|書き換|決定|切り詰|広げ|解釈)`),
		suggestion: `"無断で" or "暗黙に"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:完了|終了|終わる)[^。\n]{0,5}まで見[るて]`),
		suggestion: `"完了まで待機する" or "終了を監視する"`,
	},
	{
		pattern:    regexp.MustCompile(`(?:読まれ|見落とされ)ないと困る`),
		suggestion: `"見落とされると支障をきたす" or "読まれないと問題が生じる"`,
	},
	{
		pattern:    regexp.MustCompile(`たぶん(?:動く|入る|大丈夫|問題ない)`),
		suggestion: `"推測で済ませない" or "確証なく仮定しない"`,
	},
	{
		pattern:    regexp.MustCompile(`に留め[るてた]`),
		suggestion: `"にとどめる"`,
	},
	{
		pattern:    regexp.MustCompile(`正直な(?:集計|計算|記録)`),
		suggestion: `"精確な集計" or "客観的な集計"`,
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
