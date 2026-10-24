package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metarang/sitemap-generator-service/internal/service"
)

const sitemapFile = "fa/sitemap.xml"

func TestFileWriterWriteRemove(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "export")
	w, err := service.NewFileWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write(sitemapFile, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(sitemapFile, []byte("second")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, sitemapFile))
	if err != nil || string(got) != "second" {
		t.Fatalf("read written sitemap: %q, %v", got, err)
	}
	if err := w.Remove(sitemapFile); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(sitemapFile); err != nil {
		t.Fatalf("remove missing sitemap: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, sitemapFile)); !os.IsNotExist(err) {
		t.Fatalf("sitemap still exists: %v", err)
	}
}

func TestFileWriterRejectsInvalidNames(t *testing.T) {
	w, err := service.NewFileWriter(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "../outside.xml", "a..xml", "fa/a..xml", "fa/sub/sitemap.xml"} {
		t.Run(name, func(t *testing.T) {
			if err := w.Write(name, []byte("x")); err == nil {
				t.Fatal("write accepted invalid name")
			}
			if err := w.Remove(name); err == nil {
				t.Fatal("remove accepted invalid name")
			}
		})
	}
}

func TestFileWriterErrors(t *testing.T) {
	if _, err := service.NewFileWriter(""); err == nil {
		t.Fatal("empty export directory accepted")
	}
	root := t.TempDir()
	blocked := filepath.Join(root, "file")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.NewFileWriter(filepath.Join(blocked, "child")); err == nil {
		t.Fatal("directory under file accepted")
	}
	w, err := service.NewFileWriter(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "fa"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "fa", "occupied.xml.tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := w.Write("fa/occupied.xml", []byte("x")); err == nil || !strings.Contains(err.Error(), "write temp") {
		t.Fatalf("unexpected write error: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "fa", "directory.xml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fa", "directory.xml", "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove("fa/directory.xml"); err == nil {
		t.Fatal("remove directory succeeded")
	}
}
