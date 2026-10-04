// Package storage 保存上传文件。第一阶段只有本地磁盘实现，以后可替换为对象存储。
package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Storage interface {
	// Put 保存文件，key 为以 / 分隔的相对路径。
	Put(ctx context.Context, key string, data []byte) error
	// Get 读取文件内容。
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

type Local struct {
	Dir string
}

func NewLocal(dir string) *Local { return &Local{Dir: dir} }

func (l *Local) path(key string) (string, error) {
	if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return "", fmt.Errorf("非法的文件路径 %q", key)
	}
	return filepath.Join(l.Dir, filepath.FromSlash(key)), nil
}

// Put 先写临时文件再改名，避免读到写了一半的文件。
func (l *Local) Put(_ context.Context, key string, data []byte) error {
	dst, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("创建上传目录：%w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".upload-*")
	if err != nil {
		return fmt.Errorf("创建临时文件：%w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("写入文件：%w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("写入文件：%w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("设置文件权限：%w", err)
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return fmt.Errorf("保存文件：%w", err)
	}
	return nil
}

// Get 读取文件内容。
func (l *Local) Get(_ context.Context, key string) ([]byte, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("读取文件：%w", err)
	}
	return data, nil
}

func (l *Local) Delete(_ context.Context, key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除文件：%w", err)
	}
	return nil
}
