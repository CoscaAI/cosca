package filesystem

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── New / Options ──────────────────────────────────────────────────────

func TestNewFilesystem(t *testing.T) {
	t.Parallel()

	fs := New()
	require.NotNil(t, fs, "New() should not return nil")
	assert.True(t, fs.backups, "backups should be enabled by default")
}

func TestWithLogger(t *testing.T) {
	t.Parallel()

	fs := New(WithBackups(false))
	require.NotNil(t, fs)
}

func TestWithBackups(t *testing.T) {
	t.Parallel()

	fs := New(WithBackups(false))
	assert.False(t, fs.backups, "backups should be disabled")

	fs2 := New(WithBackups(true))
	assert.True(t, fs2.backups, "backups should be enabled")
}

func TestMultipleOptions(t *testing.T) {
	t.Parallel()

	fs := New(WithBackups(false), WithBackups(true))
	assert.True(t, fs.backups, "last option should win")
}

// ── FileExists / DirExists ─────────────────────────────────────────────

func TestFileExists(t *testing.T) {
	t.Parallel()

	assert.False(t, FileExists("/nonexistent/path/file.txt"), "nonexistent file")
	assert.False(t, FileExists("/"), "/ is a directory, not a file")
}

func TestFileExists_ExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(fpath, []byte("data"), 0o644))
	assert.True(t, FileExists(fpath))
}

func TestDirExists(t *testing.T) {
	t.Parallel()

	assert.False(t, DirExists("/nonexistent_path_xyz"), "nonexistent dir")
	assert.False(t, DirExists("/nonexistent/file.txt"), "nonexistent path")
}

func TestDirExists_ExistingDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	assert.True(t, DirExists(dir))
}

// ── ReadFile ───────────────────────────────────────────────────────────

func TestReadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "readme.md")
	content := []byte("# Hello\n\nWorld")

	require.NoError(t, os.WriteFile(fpath, content, 0o644))

	fs := New()
	data, err := fs.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestReadFile_Nonexistent(t *testing.T) {
	t.Parallel()

	fs := New()
	_, err := fs.ReadFile("/nonexistent/path/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read file")
}

func TestReadFile_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(fpath, []byte{}, 0o644))

	fs := New()
	data, err := fs.ReadFile(fpath)
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestReadFile_PackageLevel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "pkg.txt")
	require.NoError(t, os.WriteFile(fpath, []byte("pkg data"), 0o644))

	data, err := ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, []byte("pkg data"), data)
}

// ── WriteFile ──────────────────────────────────────────────────────────

func TestWriteFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "output.txt")
	content := []byte("test content")

	fs := New(WithBackups(false))
	err := fs.WriteFile(fpath, content)
	require.NoError(t, err)

	// Verify written content
	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestWriteFile_CreatesDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "deep", "nested", "dir", "file.txt")
	content := []byte("deep")

	fs := New(WithBackups(false))
	err := fs.WriteFile(fpath, content)
	require.NoError(t, err)

	assert.True(t, FileExists(fpath))
	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestWriteFile_Overwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "overwrite.txt")

	fs := New(WithBackups(false))

	require.NoError(t, fs.WriteFile(fpath, []byte("v1")))
	require.NoError(t, fs.WriteFile(fpath, []byte("v2")))

	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), data)
}

func TestWriteFile_WithBackups(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "backed.txt")

	fs := New(WithBackups(true))

	// First write
	require.NoError(t, fs.WriteFile(fpath, []byte("original")))

	// Second write should create backup
	require.NoError(t, fs.WriteFile(fpath, []byte("updated")))

	// Verify content is updated
	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, []byte("updated"), data)

	// Verify backup was created
	backupDir := filepath.Join(dir, ".backups")
	assert.True(t, DirExists(backupDir), "backup directory should exist")
}

func TestWriteFile_BackupsDisabled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "noback.txt")

	fs := New(WithBackups(false))

	require.NoError(t, fs.WriteFile(fpath, []byte("first")))
	require.NoError(t, fs.WriteFile(fpath, []byte("second")))

	// No backup dir should be created when backups are disabled
	backupDir := filepath.Join(dir, ".backups")
	_, err := os.Stat(backupDir)
	assert.True(t, os.IsNotExist(err), "backup directory should not exist")
}

func TestWriteFile_LargeContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "large.bin")
	content := make([]byte, 100*1024) // 100KB
	for i := range content {
		content[i] = byte(i % 256)
	}

	fs := New(WithBackups(false))
	err := fs.WriteFile(fpath, content)
	require.NoError(t, err)

	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestWriteFile_PackageLevel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "pkg_write.txt")

	err := WriteFile(fpath, []byte("package level"))
	require.NoError(t, err)

	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, []byte("package level"), data)
}

// ── SafeWrite ──────────────────────────────────────────────────────────

func TestSafeWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "safe.txt")

	fs := New()

	// First write — no backup needed
	require.NoError(t, fs.SafeWrite(fpath, []byte("original")))

	// Verify content
	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, []byte("original"), data)

	// Second write — should create backup
	require.NoError(t, fs.SafeWrite(fpath, []byte("updated")))

	data, err = os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, []byte("updated"), data)
}

func TestSafeWrite_NewFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "new_safe.txt")

	fs := New()

	// SafeWrite on a new file should work without backup
	require.NoError(t, fs.SafeWrite(fpath, []byte("new content")))

	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, []byte("new content"), data)
}

func TestSafeWrite_PackageLevel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "pkg_safe.txt")

	err := SafeWrite(fpath, []byte("safe package"))
	require.NoError(t, err)

	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	assert.Equal(t, []byte("safe package"), data)
}

// ── CopyFile ───────────────────────────────────────────────────────────

func TestCopyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")
	content := []byte("copy me")

	require.NoError(t, os.WriteFile(src, content, 0o644))

	fs := New()
	err := fs.CopyFile(src, dst)
	require.NoError(t, err)

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	// Source should still exist
	assert.True(t, FileExists(src))
}

func TestCopyFile_NonexistentSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fs := New()

	err := fs.CopyFile("/nonexistent/src.txt", filepath.Join(dir, "dst.txt"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open source")
}

func TestCopyFile_CreatesDestDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "nested", "deep", "dest.txt")

	require.NoError(t, os.WriteFile(src, []byte("data"), 0o644))

	fs := New()
	err := fs.CopyFile(src, dst)
	require.NoError(t, err)

	assert.True(t, FileExists(dst))
}

func TestCopyFile_BinaryContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "binary.bin")
	dst := filepath.Join(dir, "binary_copy.bin")
	content := []byte{0x00, 0xFF, 0xAB, 0xCD, 0x01, 0x02}

	require.NoError(t, os.WriteFile(src, content, 0o644))

	fs := New()
	err := fs.CopyFile(src, dst)
	require.NoError(t, err)

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestCopyFile_PackageLevel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "pkg_src.txt")
	dst := filepath.Join(dir, "pkg_dst.txt")

	require.NoError(t, os.WriteFile(src, []byte("pkg copy"), 0o644))

	err := CopyFile(src, dst)
	require.NoError(t, err)

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, []byte("pkg copy"), data)
}

// ── MoveFile ───────────────────────────────────────────────────────────

func TestMoveFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "move_src.txt")
	dst := filepath.Join(dir, "move_dst.txt")
	content := []byte("move me")

	require.NoError(t, os.WriteFile(src, content, 0o644))

	fs := New()
	err := fs.MoveFile(src, dst)
	require.NoError(t, err)

	// Destination should exist
	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	// Source should no longer exist
	assert.False(t, FileExists(src))
}

func TestMoveFile_NonexistentSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fs := New()

	err := fs.MoveFile("/nonexistent/src.txt", filepath.Join(dir, "dst.txt"))
	require.Error(t, err)
}

func TestMoveFile_CreatesDestDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "move_src.txt")
	dst := filepath.Join(dir, "nested", "move_dst.txt")

	require.NoError(t, os.WriteFile(src, []byte("move data"), 0o644))

	fs := New()
	err := fs.MoveFile(src, dst)
	require.NoError(t, err)

	assert.True(t, FileExists(dst))
	assert.False(t, FileExists(src))
}

func TestMoveFile_PackageLevel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "pkg_move_src.txt")
	dst := filepath.Join(dir, "pkg_move_dst.txt")

	require.NoError(t, os.WriteFile(src, []byte("pkg move"), 0o644))

	err := MoveFile(src, dst)
	require.NoError(t, err)

	assert.True(t, FileExists(dst))
	assert.False(t, FileExists(src))
}

// ── EnsureDir ──────────────────────────────────────────────────────────

func TestEnsureDir(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	path := filepath.Join(base, "newdir")

	fs := New()
	err := fs.EnsureDir(path)
	require.NoError(t, err)
	assert.True(t, DirExists(path))
}

func TestEnsureDir_AlreadyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	fs := New()

	// Should succeed even if already exists
	err := fs.EnsureDir(dir)
	require.NoError(t, err)
	assert.True(t, DirExists(dir))
}

func TestEnsureDir_Nested(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	path := filepath.Join(base, "a", "b", "c")

	fs := New()
	err := fs.EnsureDir(path)
	require.NoError(t, err)
	assert.True(t, DirExists(path))
}

func TestEnsureDir_PackageLevel(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "pkg_dir")

	err := EnsureDir(dir)
	require.NoError(t, err)
	assert.True(t, DirExists(dir))
}

// ── ListFiles ──────────────────────────────────────────────────────────

func TestListFiles_NoPattern(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.go"), []byte("c"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))

	fs := New()
	files, err := fs.ListFiles(dir, "")
	require.NoError(t, err)

	// Should return only files (not directories)
	assert.Len(t, files, 3)
}

func TestListFiles_WithPattern(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file1.go"), []byte("1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file2.go"), []byte("2"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte("readme"), 0o644))

	fs := New()
	files, err := fs.ListFiles(dir, "*.go")
	require.NoError(t, err)
	assert.Len(t, files, 2)

	for _, f := range files {
		assert.True(t, strings.HasSuffix(f, ".go"), "should match .go extension")
	}
}

func TestListFiles_EmptyDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	fs := New()
	files, err := fs.ListFiles(dir, "")
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestListFiles_NonexistentDirectory(t *testing.T) {
	t.Parallel()

	fs := New()
	_, err := fs.ListFiles("/nonexistent/dir", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory does not exist")
}

func TestListFiles_OnlyDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "dir1"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "dir2"), 0o755))

	fs := New()
	files, err := fs.ListFiles(dir, "")
	require.NoError(t, err)
	assert.Empty(t, files, "directories should not be listed")
}

func TestListFiles_PatternNoMatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte("x"), 0o644))

	fs := New()
	files, err := fs.ListFiles(dir, "*.go")
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestListFiles_PackageLevel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.go"), []byte("pkg"), 0o644))

	files, err := ListFiles(dir, "*.go")
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

// ── GetFileHash ────────────────────────────────────────────────────────

func TestGetFileHash(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "hashme.txt")
	require.NoError(t, os.WriteFile(fpath, []byte("hello"), 0o644))

	fs := New()
	hash, err := fs.GetFileHash(fpath)
	require.NoError(t, err)
	assert.Len(t, hash, 64) // SHA-256 hex is 64 chars

	// Known SHA-256 of "hello"
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", hash)
}

func TestGetFileHash_Nonexistent(t *testing.T) {
	t.Parallel()

	fs := New()
	_, err := fs.GetFileHash("/nonexistent/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open file for hash")
}

func TestGetFileHash_Consistency(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "consistent.txt")
	require.NoError(t, os.WriteFile(fpath, []byte("same data"), 0o644))

	fs := New()
	h1, _ := fs.GetFileHash(fpath)
	h2, _ := fs.GetFileHash(fpath)

	assert.Equal(t, h1, h2, "hash should be consistent")
}

func TestGetFileHash_PackageLevel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "pkg_hash.txt")
	require.NoError(t, os.WriteFile(fpath, []byte("hash"), 0o644))

	hash, err := GetFileHash(fpath)
	require.NoError(t, err)
	assert.Len(t, hash, 64)
}

// ── GetFileInfo ────────────────────────────────────────────────────────

func TestGetFileInfo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "info.txt")
	content := []byte("info content")
	require.NoError(t, os.WriteFile(fpath, content, 0o644))

	fs := New()
	fi, err := fs.GetFileInfo(fpath)
	require.NoError(t, err)

	assert.Equal(t, fpath, fi.Path)
	assert.Equal(t, "info.txt", fi.Name)
	assert.Equal(t, int64(len(content)), fi.Size)
	assert.False(t, fi.IsDir)
	assert.NotEmpty(t, fi.SHA256Hash)
	assert.Len(t, fi.SHA256Hash, 64)
}

func TestGetFileInfo_Directory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	fs := New()
	fi, err := fs.GetFileInfo(dir)
	require.NoError(t, err)

	assert.Equal(t, dir, fi.Path)
	assert.True(t, fi.IsDir)
	assert.Empty(t, fi.SHA256Hash, "directories should not have a hash")
}

func TestGetFileInfo_Nonexistent(t *testing.T) {
	t.Parallel()

	fs := New()
	_, err := fs.GetFileInfo("/nonexistent/file.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stat file")
}

func TestGetFileInfo_PackageLevel(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "pkg_info.txt")
	require.NoError(t, os.WriteFile(fpath, []byte("info"), 0o644))

	fi, err := GetFileInfo(fpath)
	require.NoError(t, err)
	assert.Equal(t, "pkg_info.txt", fi.Name)
	assert.Len(t, fi.SHA256Hash, 64)
}

// ── FileInfo struct ────────────────────────────────────────────────────

func TestFileInfo(t *testing.T) {
	t.Parallel()

	fi := FileInfo{
		Path:  "/test/file.go",
		Name:  "file.go",
		Size:  100,
		IsDir: false,
	}

	assert.Equal(t, "/test/file.go", fi.Path)
	assert.Equal(t, "file.go", fi.Name)
	assert.Equal(t, int64(100), fi.Size)
	assert.False(t, fi.IsDir)
}

func TestFileInfo_DirectoryFlag(t *testing.T) {
	t.Parallel()

	fi := FileInfo{
		Path:  "/test/dir",
		Name:  "dir",
		IsDir: true,
	}

	assert.True(t, fi.IsDir)
	assert.Equal(t, "dir", fi.Name)
}

// ── HashBytes ──────────────────────────────────────────────────────────

func TestHashBytes(t *testing.T) {
	t.Parallel()

	h1 := HashBytes([]byte("hello"))
	h2 := HashBytes([]byte("hello"))
	h3 := HashBytes([]byte("world"))

	assert.Equal(t, h1, h2, "same data should produce same hash")
	assert.NotEqual(t, h1, h3, "different data should produce different hash")
	assert.Len(t, h1, 64, "hash should be 64 hex characters")
}

func TestHashBytes_Empty(t *testing.T) {
	t.Parallel()

	h := HashBytes([]byte{})
	assert.Len(t, h, 64)
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", h)
}

func TestHashBytes_Nil(t *testing.T) {
	t.Parallel()

	h := HashBytes(nil)
	assert.Len(t, h, 64)
	assert.Equal(t, HashBytes([]byte{}), h)
}

// ── UniqueTempPath ─────────────────────────────────────────────────────

func TestUniqueTempPath(t *testing.T) {
	t.Parallel()

	p1 := UniqueTempPath("test")
	p2 := UniqueTempPath("test")
	assert.NotEqual(t, p1, p2, "temp paths should be unique")
}

func TestUniqueTempPath_ContainsPrefix(t *testing.T) {
	t.Parallel()

	p := UniqueTempPath("myapp")
	assert.Contains(t, p, "myapp")
	assert.Contains(t, p, os.TempDir())
}

// ── IsAbsPath ──────────────────────────────────────────────────────────

func TestIsAbsPath(t *testing.T) {
	// Semântica POSIX: "/absolute/path" e "/" são absolutos apenas em
	// sistemas com separador "/". No Windows filepath.IsAbs("/etc") é false
	// (o separador é "\"), então o caso POSIX não se aplica.
	if runtime.GOOS == "windows" {
		t.Skip("semântica de path POSIX (separador '/') não se aplica no Windows")
	}
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{"/absolute/path", true},
		{"/", true},
		{"relative/path", false},
		{"justname", false},
		{"./relative", false},
		{"../relative", false},
		{"", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			got := IsAbsPath(tc.path)
			assert.Equal(t, tc.want, got, "IsAbsPath(%q)", tc.path)
		})
	}
}

// ── ResolvePath ────────────────────────────────────────────────────────

func TestResolvePath(t *testing.T) {
	// As expectativas são POSIX ("/abs/path", "/base/relative" com separador
	// "/"). No Windows filepath.Join/Clean produzem "\base\relative", então o
	// caso POSIX não se aplica.
	if runtime.GOOS == "windows" {
		t.Skip("semântica de path POSIX (separador '/') não se aplica no Windows")
	}
	t.Parallel()

	tests := []struct {
		path string
		base string
		want string
	}{
		{"/abs/path", "/base", "/abs/path"},
		{"relative", "/base", "/base/relative"},
		{"sub/dir", "/base", "/base/sub/dir"},
		{"./current", "/base", "/base/current"},
		{"../parent", "/base", "/parent"},
		{"relative/../sibling", "/base", "/base/sibling"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := ResolvePath(tc.path, tc.base)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolvePath_HomeExpansion(t *testing.T) {
	t.Parallel()

	// ~/ expansion should resolve to the user's home directory
	result := ResolvePath("~/documents/file.txt", "/base")
	assert.NotContains(t, result, "~", "tilde should be expanded")
	assert.True(t, IsAbsPath(result), "result should be absolute")
}

// ── Path Constants ─────────────────────────────────────────────────────

func TestPathConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ".config/cosca", DefaultCoscaHome)
	assert.Equal(t, "runtime", RuntimeDirName)
	assert.Equal(t, "data", DataDirName)
	assert.Equal(t, "cache", CacheDirName)
	assert.Equal(t, "logs", LogsDirName)
	assert.Equal(t, "tmp", TempDirName)
	assert.Equal(t, "plugins", PluginsDirName)
	assert.Equal(t, "backups", BackupsDirName)
	assert.Equal(t, ".cosca", ProjectDirName)
	assert.Equal(t, "cosca.pid", PidFileName)
	assert.Equal(t, "telemetry.db", TelemetryDBName)
	assert.Equal(t, "knowledge.db", KnowledgeDBName)
}

// ── EnsureDirectories ──────────────────────────────────────────────────

func TestEnsureDirectories(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	base := t.TempDir()

	// Override COSCA_HOME to use temp dir
	t.Setenv("COSCA_HOME", base)
	// Clear other env vars that might interfere
	t.Setenv("COSCA_RUNTIME_DIR", "")
	t.Setenv("COSCA_DATA_DIR", "")
	t.Setenv("COSCA_CACHE_DIR", "")
	t.Setenv("COSCA_LOGS_DIR", "")
	t.Setenv("COSCA_TEMP_DIR", "")
	t.Setenv("COSCA_PLUGINS_DIR", "")
	t.Setenv("COSCA_BACKUPS_DIR", "")

	dirs, err := EnsureDirectories()
	require.NoError(t, err)
	require.NotEmpty(t, dirs)

	for name, path := range dirs {
		assert.True(t, DirExists(path), "directory %s (%s) should exist", name, path)
	}
}

func TestEnsureDirectories_AllKeys(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	base := t.TempDir()
	t.Setenv("COSCA_HOME", base)

	dirs, err := EnsureDirectories()
	require.NoError(t, err)

	expectedKeys := []string{"home", "runtime", "data", "cache", "logs", "temp", "plugins", "backups"}
	for _, key := range expectedKeys {
		_, ok := dirs[key]
		assert.True(t, ok, "key %s should be present", key)
	}
}

// ── Platform Helpers ──────────────────────────────────────────────────

func TestDataHome(t *testing.T) {
	t.Parallel()

	result := DataHome()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "cosca")
}

func TestConfigHome(t *testing.T) {
	t.Parallel()

	result := ConfigHome()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "cosca")
}

func TestCacheHome(t *testing.T) {
	t.Parallel()

	result := CacheHome()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "cosca")
}

// ── Cosca Directory Functions ────────────────────────────────────────────

func TestCoscaHomeDir(t *testing.T) {
	t.Parallel()

	result := CoscaHomeDir()
	assert.NotEmpty(t, result)
	// filepath.Join evita hardcodar o separador: no Windows o resultado é
	// "C:\Users\<user>\.config\cosca", não ".config/cosca".
	assert.Contains(t, result, filepath.Join(".config", "cosca"))
}

func TestCoscaHomeDir_EnvOverride(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	custom := t.TempDir()
	t.Setenv("COSCA_HOME", custom)

	result := CoscaHomeDir()
	assert.Equal(t, custom, result)
}

func TestCoscaRuntimeDir(t *testing.T) {
	t.Parallel()

	result := CoscaRuntimeDir()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "runtime")
}

func TestCoscaRuntimeDir_EnvOverride(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	custom := t.TempDir()
	t.Setenv("COSCA_RUNTIME_DIR", custom)

	result := CoscaRuntimeDir()
	assert.Equal(t, custom, result)
}

func TestCoscaDataDir(t *testing.T) {
	t.Parallel()

	result := CoscaDataDir()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "data")
}

func TestCoscaCacheDir(t *testing.T) {
	t.Parallel()

	result := CoscaCacheDir()
	assert.NotEmpty(t, result)
}

func TestCoscaLogsDir(t *testing.T) {
	t.Parallel()

	result := CoscaLogsDir()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "logs")
}

func TestCoscaTempDir(t *testing.T) {
	t.Parallel()

	result := CoscaTempDir()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "tmp")
}

func TestCoscaPluginsDir(t *testing.T) {
	t.Parallel()

	result := CoscaPluginsDir()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "plugins")
}

func TestCoscaBackupsDir(t *testing.T) {
	t.Parallel()

	result := CoscaBackupsDir()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "backups")
}

func TestCoscaProjectDir(t *testing.T) {
	t.Parallel()

	result := CoscaProjectDir()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, ".cosca")
}

func TestCoscaProjectDir_EnvOverride(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv
	custom := t.TempDir()
	t.Setenv("COSCA_PROJECT_DIR", custom)

	result := CoscaProjectDir()
	assert.Equal(t, custom, result)
}

func TestCoscaGlobalDir(t *testing.T) {
	t.Parallel()

	result := CoscaGlobalDir()
	assert.NotEmpty(t, result)
}

// ── Composed Path Functions ────────────────────────────────────────────

func TestCoscaPIDFile(t *testing.T) {
	t.Parallel()

	result := CoscaPIDFile()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "cosca.pid")
}

func TestTelemetryDBPath(t *testing.T) {
	t.Parallel()

	result := TelemetryDBPath()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "telemetry.db")
}

func TestKnowledgeDBPath(t *testing.T) {
	t.Parallel()

	result := KnowledgeDBPath()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "knowledge.db")
}

func TestConfigFilePath(t *testing.T) {
	t.Parallel()

	result := ConfigFilePath()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "config.yaml")
}

func TestProjectConfigFilePath(t *testing.T) {
	t.Parallel()

	result := ProjectConfigFilePath()
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "config.yaml")
}

// ── createBackup (indirectly tested via WriteFile) ─────────────────────

func TestWriteFile_BackupCreatedCorrectly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fpath := filepath.Join(dir, "backup_test.txt")

	fs := New(WithBackups(true))

	require.NoError(t, fs.WriteFile(fpath, []byte("original content")))
	require.NoError(t, fs.WriteFile(fpath, []byte("updated content")))

	// Verify backup directory exists
	backupDir := filepath.Join(dir, ".backups")
	assert.True(t, DirExists(backupDir))

	// Verify at least one backup file exists
	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "backup directory should contain backups")
}
