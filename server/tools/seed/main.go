// Package main 是开发环境用的压测数据工具：向开发库写入大量内容，用于压测和 Lighthouse 实测。
// 独立于正式程序，不参与构建产物；开头会检查库名，避免误连生产库。
//
// 用法：cd server && go run ./tools/seed -count 10000
//
//	-clean 只删除以前生成的压测内容
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"xzji/internal/config"
	platformdb "xzji/internal/platform/db"
)

// titlePrefix 生成内容的标题前缀，便于识别和清理
const titlePrefix = "【压测】"

// batch 每批写入的内容数
const batch = 500

func main() {
	count := flag.Int("count", 10000, "生成的内容数量")
	cfgPath := flag.String("c", "config.yaml", "配置文件路径")
	clean := flag.Bool("clean", false, "只删除以前生成的压测内容，不生成新数据")
	flag.Parse()

	if err := run(*cfgPath, *count, *clean); err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
}

func run(cfgPath string, count int, clean bool) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	ctx := context.Background()
	db, err := platformdb.Open(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()

	var name string
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		return err
	}
	if !strings.HasSuffix(name, "_dev") {
		return fmt.Errorf("只允许写开发库（库名以 _dev 结尾），当前连接的是 %q", name)
	}
	if clean {
		return cleanSeeded(ctx, db)
	}

	authorID, err := firstAdmin(ctx, db)
	if err != nil {
		return err
	}
	categories, err := ids(ctx, db, "SELECT id FROM categories")
	if err != nil {
		return err
	}
	if len(categories) == 0 {
		return errors.New("还没有分类，请先在后台建几个分类")
	}
	tags, err := ids(ctx, db, "SELECT id FROM tags")
	if err != nil {
		return err
	}
	media, err := ids(ctx, db, "SELECT id FROM media")
	if err != nil {
		return err
	}

	start := time.Now()
	rnd := rand.New(rand.NewSource(20261004))
	for done := 0; done < count; done += batch {
		size := min(batch, count-done)
		if err := seedBatch(ctx, db, rnd, done, size, authorID, categories, tags, media); err != nil {
			return err
		}
		fmt.Printf("\r已写入 %d/%d 篇", done+size, count)
	}
	fmt.Printf("\n完成：%d 篇，用时 %s\n", count, time.Since(start).Round(time.Second))
	return nil
}

// seeded 一批内容里每篇的附加信息，用来自动生成附属数据。
type seeded struct {
	id     int64
	hidden bool
}

// seedBatch 写一批内容及其标签、资源信息、下载地址和隐藏块。批次内自增 ID 连续。
func seedBatch(ctx context.Context, db *sql.DB, rnd *rand.Rand, seq, size int, authorID int64, categories, tags, media []int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	values := strings.Builder{}
	args := make([]any, 0, size*20)
	posts := make([]seeded, 0, size)
	for i := range size {
		if i > 0 {
			values.WriteString(",")
		}
		values.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")

		n := seq + i
		kind := "resource"
		if rnd.Intn(10) < 4 {
			kind = "article"
		}
		body, toc, hidden := contentHTML(rnd, n)
		status := "published"
		switch rnd.Intn(100) {
		case 0:
			status = "draft"
		case 1:
			status = "trash"
		}
		// 少数内容定时到未来发布，用来验证「未到发布时间不出现」
		published := time.Now().UTC().Add(-time.Duration(rnd.Intn(365*24)) * time.Hour)
		if rnd.Intn(100) == 0 {
			published = time.Now().UTC().Add(time.Duration(rnd.Intn(48)+1) * time.Hour)
		}
		price := int64(0)
		if kind == "resource" && rnd.Intn(3) == 0 {
			price = int64(rnd.Intn(20)+1) * 100
		}
		args = append(args,
			kind, fmt.Sprintf("%s%s %05d", titlePrefix, subject(rnd, kind), n), "", 1, body, toc,
			nullable(pick(rnd, media)), categories[rnd.Intn(len(categories))], authorID, status, 0,
			price, rnd.Intn(3), 1, "", "", summary(rnd, kind),
			published, rnd.Intn(20000), rnd.Intn(30),
		)
		posts = append(posts, seeded{hidden: hidden})
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO posts (
		type, title, summary, summary_auto, content_html, toc, cover_media_id, category_id, author_id,
		status, pinned, price, member_free_rank, member_discount, seo_title, seo_keywords, seo_description,
		published_at, view_count, comment_count) VALUES `+values.String(), args...)
	if err != nil {
		return fmt.Errorf("写入内容：%w", err)
	}
	firstID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for i := range posts {
		posts[i].id = firstID + int64(i)
	}

	if err := seedChildren(ctx, tx, rnd, posts, tags, media); err != nil {
		return err
	}
	return tx.Commit()
}

// seedChildren 写入附属数据：分类标签、资源信息、下载地址、隐藏块。
func seedChildren(ctx context.Context, tx *sql.Tx, rnd *rand.Rand, posts []seeded, tags, media []int64) error {
	res := &strings.Builder{}
	resArgs := []any{}
	links := &strings.Builder{}
	linkArgs := []any{}
	blocks := &strings.Builder{}
	blockArgs := []any{}
	postTags := &strings.Builder{}
	tagArgs := []any{}
	platforms := []string{"Windows", "macOS", "Android", "iOS", "Linux"}

	for _, p := range posts {
		if rnd.Intn(3) > 0 {
			if res.Len() > 0 {
				res.WriteString(",")
			}
			res.WriteString("(?,?,?,?,?,?,?,?)")
			resArgs = append(resArgs, p.id, platforms[rnd.Intn(len(platforms))],
				fmt.Sprintf("%d.%d.%d", rnd.Intn(10)+1, rnd.Intn(20), rnd.Intn(20)),
				fmt.Sprintf("%d.%d MB", rnd.Intn(500)+1, rnd.Intn(10)), nullable(pick(rnd, media)), nil, nil, "public")
			for l := range 2 {
				if links.Len() > 0 {
					links.WriteString(",")
				}
				links.WriteString("(?,?,?,?,?,?,?)")
				linkArgs = append(linkArgs, p.id, fmt.Sprintf("下载地址 %d", l+1),
					fmt.Sprintf("https://pan.example.com/s/%d-%d", p.id, l), "", "", l, "ok")
			}
		}
		if p.hidden {
			if blocks.Len() > 0 {
				blocks.WriteString(",")
			}
			blocks.WriteString("(?,?,?,?,?)")
			blockArgs = append(blockArgs, p.id, 1, "paid", "付费可见",
				fmt.Sprintf("<p>隐藏内容：本文档的补充说明，编号 %d。登录并购买后可以查看。</p>", p.id))
		}
		for t := range 1 + rnd.Intn(max(1, min(3, len(tags)))) {
			if len(tags) == 0 {
				break
			}
			if postTags.Len() > 0 {
				postTags.WriteString(",")
			}
			postTags.WriteString("(?,?,?)")
			tagArgs = append(tagArgs, p.id, tags[(int(p.id)+t)%len(tags)], t)
		}
	}
	if res.Len() > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO resources (post_id, platform, version, file_size, icon_media_id, attrs, changelog, download_cond) VALUES `+res.String(), resArgs...); err != nil {
			return fmt.Errorf("写入资源信息：%w", err)
		}
	}
	if links.Len() > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO resource_links (post_id, name, url, extract_code, unzip_password, sort, status) VALUES `+links.String(), linkArgs...); err != nil {
			return fmt.Errorf("写入下载地址：%w", err)
		}
	}
	if blocks.Len() > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO post_hidden_blocks (post_id, seq, cond, title, content_html) VALUES `+blocks.String(), blockArgs...); err != nil {
			return fmt.Errorf("写入隐藏块：%w", err)
		}
	}
	if postTags.Len() > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO post_tags (post_id, tag_id, sort) VALUES `+postTags.String(), tagArgs...); err != nil {
			return fmt.Errorf("写入内容标签：%w", err)
		}
	}
	return nil
}

// cleanSeeded 删除以前生成的压测内容及其附属数据。表之间没有外键，要按依赖顺序逐个删。
func cleanSeeded(ctx context.Context, db *sql.DB) error {
	ids, err := ids(ctx, db, "SELECT id FROM posts WHERE title LIKE ?", titlePrefix+"%")
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		fmt.Println("没有找到压测数据")
		return nil
	}
	tables := []string{"comments", "download_logs", "link_reports", "post_tags", "post_hidden_blocks", "resource_links", "resources"}
	for start := 0; start < len(ids); start += 1000 {
		part := ids[start:min(start+1000, len(ids))]
		list := strings.TrimSuffix(strings.Repeat("?,", len(part)), ",")
		args := make([]any, len(part))
		for i, id := range part {
			args[i] = id
		}
		for _, tbl := range tables {
			if _, err := db.ExecContext(ctx, "DELETE FROM "+tbl+" WHERE post_id IN ("+list+")", args...); err != nil {
				return fmt.Errorf("清理 %s：%w", tbl, err)
			}
		}
		if _, err := db.ExecContext(ctx, "DELETE FROM posts WHERE id IN ("+list+")", args...); err != nil {
			return fmt.Errorf("清理内容：%w", err)
		}
	}
	fmt.Printf("已删除 %d 篇压测内容\n", len(ids))
	return nil
}

type tocItem struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Level int    `json:"level"`
}

// contentHTML 生成正文：标题锚点、段落，以及可能有的一处隐藏块占位符。
// 格式与编辑器保存的一致（标题 id 为 h-N，隐藏块用 data-hidden-block 占位）。
func contentHTML(rnd *rand.Rand, n int) (body, toc string, hidden bool) {
	var b strings.Builder
	var items []tocItem
	sections := 2 + rnd.Intn(3)
	hidden = rnd.Intn(2) == 0
	hiddenAt := rnd.Intn(sections)
	for i := range sections {
		id := fmt.Sprintf("h-%d", i+1)
		text := fmt.Sprintf("第 %d 部分：%s", i+1, subject(rnd, "article"))
		fmt.Fprintf(&b, `<h2 id="%s">%s</h2>`, id, text)
		items = append(items, tocItem{ID: id, Text: text, Level: 2})
		for range 1 + rnd.Intn(3) {
			fmt.Fprintf(&b, "<p>%s</p>", paragraph(n))
		}
		if hidden && i == hiddenAt {
			b.WriteString(`<div data-hidden-block="1"></div>`)
		}
	}
	raw, _ := json.Marshal(items)
	return b.String(), string(raw), hidden
}

func subject(rnd *rand.Rand, kind string) string {
	words := []string{"批量处理", "快捷键", "界面美化", "数据备份", "格式转换", "自动化脚本", "权限管理", "离线使用", "多端同步", "性能调优"}
	if kind == "resource" {
		return "工具 " + words[rnd.Intn(len(words))]
	}
	return "教程 " + words[rnd.Intn(len(words))]
}

func paragraph(n int) string {
	return fmt.Sprintf("这是第 %d 篇内容的说明段落，用来撑起正文长度，模拟真实的排版和文字量，页面渲染时按段落逐个处理。文中提到的方法适用于大多数场景，按步骤操作即可。", n)
}

func summary(rnd *rand.Rand, kind string) string {
	return fmt.Sprintf("%s的简介，包含主要功能、使用方法和小提示。", subject(rnd, kind))
}

func firstAdmin(ctx context.Context, db *sql.DB) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx, "SELECT id FROM users WHERE role = 'admin' ORDER BY id LIMIT 1").Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errors.New("还没有管理员账号，先运行 xzji admin create")
	}
	return id, err
}

func ids(ctx context.Context, db *sql.DB, query string, args ...any) ([]int64, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func nullable(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func pick(rnd *rand.Rand, list []int64) int64 {
	if len(list) == 0 {
		return 0
	}
	return list[rnd.Intn(len(list))]
}
