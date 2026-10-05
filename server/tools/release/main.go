// Command release 把构建产物打成部署包，输出到 dist/。
// 用 Go 标准库打包，Windows 上也能跑，不依赖 tar 命令。
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// entry 包里的一个文件。mode 会写进 tar 头，可执行权限靠它带到 Linux。
type entry struct {
	src  string
	dst  string
	mode int64
}

func main() {
	version := flag.String("version", "", "版本号，用在包名里，例如 v1.0.0 或 7f08460")
	root := flag.String("root", "..", "仓库根目录")
	out := flag.String("out", "../dist", "输出目录")
	flag.Parse()

	if *version == "" {
		fail("必须指定 -version")
	}
	dist := filepath.Clean(*out)

	if err := os.MkdirAll(dist, 0o755); err != nil {
		fail("创建输出目录：%v", err)
	}

	pkgs := []struct {
		name  string
		build func(string) ([]entry, error)
	}{
		{fmt.Sprintf("xzji-server-%s-linux-amd64.tar.gz", *version), serverPackage},
		{fmt.Sprintf("xzji-web-%s.tar.gz", *version), webPackage},
	}
	for _, p := range pkgs {
		entries, err := p.build(*root)
		if err != nil {
			fail("%v", err)
		}
		dst := filepath.Join(dist, p.name)
		if err := writePackage(dst, entries); err != nil {
			fail("%v", err)
		}
		sum, err := checksum(dst)
		if err != nil {
			fail("%v", err)
		}
		if err := os.WriteFile(dst+".sha256", []byte(sum+"  "+p.name+"\n"), 0o644); err != nil {
			fail("写校验文件：%v", err)
		}
		info, err := os.Stat(dst)
		if err != nil {
			fail("%v", err)
		}
		fmt.Printf("%s  %.1fMB\n", p.name, float64(info.Size())/(1<<20))
	}
}

// serverPackage 后端包：可执行文件、配置样例、备份脚本、systemd 单元、Nginx 样例和建库 SQL。
func serverPackage(root string) ([]entry, error) {
	bin := filepath.Join(root, "server", "bin", "xzji-linux-amd64")
	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf("找不到 Linux 后端可执行文件 %s，先跑 task build:server:linux", bin)
	}
	entries := []entry{
		{bin, "bin/xzji", 0o755},
		{filepath.Join(root, "deploy", "config.prod.example.yaml"), "config.example.yaml", 0o644},
		{filepath.Join(root, "deploy", "systemd", "xzji-server.service"), "systemd/xzji-server.service", 0o644},
		{filepath.Join(root, "deploy", "sql", "create-database.sql"), "sql/create-database.sql", 0o644},
	}
	// 分机部署时后端也要照着 Nginx 样例改，所以包一份进去
	nginx, err := glob(filepath.Join(root, "deploy", "nginx"), "nginx/", 0o644)
	if err != nil {
		return nil, err
	}
	scripts, err := glob(filepath.Join(root, "deploy", "scripts"), "scripts/", 0o755)
	if err != nil {
		return nil, err
	}
	return append(append(entries, nginx...), scripts...), nil
}

// webPackage 前台包：Nuxt 构建产物、环境变量样例和 systemd 单元。
func webPackage(root string) ([]entry, error) {
	output := filepath.Join(root, "web", ".output", "server", "index.mjs")
	if _, err := os.Stat(output); err != nil {
		return nil, fmt.Errorf("找不到前台构建产物 %s，先跑 task build:web", output)
	}
	entries, err := glob(filepath.Join(root, "web", ".output"), ".output/", 0o644)
	if err != nil {
		return nil, err
	}
	return append(entries,
		entry{filepath.Join(root, "deploy", "web.env.example"), "web.env.example", 0o644},
		entry{filepath.Join(root, "deploy", "systemd", "xzji-web.service"), "systemd/xzji-web.service", 0o644},
	), nil
}

// glob 递归收集目录下的文件，保持路径稳定。
func glob(dir, prefix string, mode int64) ([]entry, error) {
	var out []entry
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		out = append(out, entry{p, prefix + filepath.ToSlash(rel), mode})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("读取目录 %s：%w", dir, err)
	}
	slices.SortFunc(out, func(a, b entry) int { return strings.Compare(a.dst, b.dst) })
	return out, nil
}

func writePackage(dst string, entries []entry) error {
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建 %s：%w", dst, err)
	}
	defer f.Close()

	gz, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(gz)

	// 先建目录项，解压后每个目录的权限都明确
	now := time.Now()
	dirs := map[string]bool{}
	for _, e := range entries {
		for _, d := range parentDirs(e.dst) {
			if dirs[d] {
				continue
			}
			dirs[d] = true
			if err := tw.WriteHeader(&tar.Header{Name: d + "/", Typeflag: tar.TypeDir, Mode: 0o755, ModTime: now}); err != nil {
				return err
			}
		}
		info, err := os.Stat(e.src)
		if err != nil {
			return fmt.Errorf("读取 %s：%w", e.src, err)
		}
		if err := tw.WriteHeader(&tar.Header{
			Name:     e.dst,
			Typeflag: tar.TypeReg,
			Mode:     e.mode,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
		}); err != nil {
			return err
		}
		in, err := os.Open(e.src)
		if err != nil {
			return fmt.Errorf("打开 %s：%w", e.src, err)
		}
		_, copyErr := io.Copy(tw, in)
		in.Close()
		if copyErr != nil {
			return fmt.Errorf("写入 %s：%w", e.dst, copyErr)
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// parentDirs 返回路径里需要提前创建的目录，从最外层开始。
func parentDirs(name string) []string {
	var out []string
	for {
		dir := path.Dir(name)
		if dir == "." || dir == "/" {
			break
		}
		out = append(out, dir)
		name = dir
	}
	slices.Reverse(out)
	return out
}

func checksum(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "打包失败："+format+"\n", args...)
	os.Exit(1)
}
