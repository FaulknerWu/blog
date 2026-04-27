package media

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Storage struct {
	root string
}

func NewStorage(root string) (*Storage, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create media dir: %w", err)
	}
	return &Storage{root: root}, nil
}

// 目录布局：{root}/{sha 前两位}/{sha}/{width}.{format}，按前缀分桶避免单目录文件过多。
func (s *Storage) dir(sha string) string { return filepath.Join(s.root, sha[:2], sha) }

func (s *Storage) path(sha string, v Variant) string {
	return filepath.Join(s.dir(sha), fmt.Sprintf("%d.%s", v.Width, v.Format))
}

// save 先写临时文件再 rename，保证读取方不会读到写了一半的文件。
func (s *Storage) save(sha string, p *processed) error {
	dir := s.dir(sha)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	for _, v := range p.variants {
		f, err := os.CreateTemp(dir, ".upload-*")
		if err != nil {
			return err
		}
		_, werr := f.Write(p.files[v.Width])
		cerr := f.Close()
		if werr != nil || cerr != nil {
			os.Remove(f.Name())
			return fmt.Errorf("write variant: %w", errors.Join(werr, cerr))
		}
		if err := os.Rename(f.Name(), s.path(sha, v)); err != nil {
			os.Remove(f.Name())
			return err
		}
	}
	return nil
}

func (s *Storage) remove(sha string) error {
	if err := os.RemoveAll(s.dir(sha)); err != nil {
		return err
	}
	_ = os.Remove(filepath.Dir(s.dir(sha))) // 分桶目录为空时一并删除，非空时失败可忽略
	return nil
}

func (s *Storage) open(sha string, v Variant) (*os.File, error) {
	return os.Open(s.path(sha, v))
}
