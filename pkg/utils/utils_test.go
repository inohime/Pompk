package utils

import (
	"fmt"
	"os"
	"path"
	"testing"
)

func TestCreateDirectory(t *testing.T) {
	dir := "dummy_dir"
	CreateDirectory(dir)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatalf("Failed to create directory '%s': %v\n", dir, err)
	}

	err := os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("Failed to remove directory '%s': %v\n", dir, err)
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	pkgDownloadURL := "http://ftp.us.debian.org/debian/pool/main/d/dbus/libdbus-1-3_1.14.10-1~deb12u1_amd64.deb"
	pkgName := path.Base(pkgDownloadURL)

	err := WriteFile(pkgDownloadURL, dir+"/"+pkgName)
	fmt.Println("File Path:", dir+"/"+pkgName)
	if err != nil {
		t.Fatal("Failed to write file to disk:", err)
	}
}

func TestRequestURL(t *testing.T) {
	pkgURL := "https://packages.debian.org/bookworm/amd64/libdbus-1-3"
	_, err := RequestURL(pkgURL)
	if err != nil {
		t.Fatal("Failed to request HTML document:", err)
	}
}

func TestSetCache(t *testing.T) {
	sc := NewCache()
	sc.Add("libdbus")

	if _, exists := sc.set.Load("libdbus"); !exists {
		t.Fatal("Failed to set key in cache")
	}
}

func TestGetCache(t *testing.T) {
	sc := NewCache()
	sc.Add("libdbus")

	if exists := sc.Get("libdbus"); !exists {
		t.Fatal("Failed to get key from cache")
	}
}
