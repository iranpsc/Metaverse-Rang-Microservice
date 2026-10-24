package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metarang/sitemap-generator-service/internal/service"
)

func TestFileWriterWriteRemove(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "export")
	w, err := service.NewFileWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write("sitemap.xml", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := w.Write("sitemap.xml", []byte("second")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "sitemap.xml"))
	if err != nil || string(got) != "second" {
		t.Fatalf("read written sitemap: %q, %v", got, err)
	}
	if err := w.Remove("sitemap.xml"); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove("sitemap.xml"); err != nil {
		t.Fatalf("remove missing sitemap: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sitemap.xml")); !os.IsNotExist(err) {
		t.Fatalf("sitemap still exists: %v", err)
	}
}

func TestFileWriterRejectsInvalidNames(t *testing.T) {
	w, err := service.NewFileWriter(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "../outside.xml", "a..xml", "sub/sitemap.xml"} {
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
	if err := os.Mkdir(filepath.Join(root, "occupied.xml.tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := w.Write("occupied.xml", []byte("x")); err == nil || !strings.Contains(err.Error(), "write temp") {
		t.Fatalf("unexpected write error: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory.xml"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "directory.xml", "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove("directory.xml"); err == nil {
		t.Fatal("remove directory succeeded")
	}
}
