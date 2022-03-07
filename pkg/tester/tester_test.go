package tester_test

import (
	"github.com/getoutreach/gobox/pkg/tester"

	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestPanic(t *testing.T) {
	tx := tester.New(tester.WithName("TestPanic"), logWriter(t))

	tester.Run(tx, "Panic Test", func(t tester.T) {
		panic("reason = panic")
	})
	r := tester.Results(tx)
	assert.Assert(t, r.Failed)
	assert.Equal(t, len(r.Failures), 1)
	assert.Equal(t, r.Failures[0].TestName, "TestPanic/Panic Test")
	assert.Equal(t, r.Failures[0].Failure, "panic reason = panic")
}

func TestCleanup(t *testing.T) {
	tx := tester.New(tester.WithName("TestCleanup"), logWriter(t))

	s := ""
	tester.Run(tx, "Panic Test", func(tx tester.T) {
		tx.Cleanup(func() { s += "cleanup 1\n" })
		tx.Cleanup(func() { s += "cleanup 2\n" })
	})

	r := tester.Results(tx)
	assert.Assert(t, !r.Failed)
	assert.Equal(t, s, "cleanup 2\ncleanup 1\n")
}

func TestLogLogf(t *testing.T) {
	messages := []string{}
	logWriter := tester.WithLogWriter(func(name, message string) {
		messages = append(messages, message)
	})
	tx := tester.New(tester.WithName("TestLogLogf"), logWriter)

	tx.Log("hello", "world")
	assert.DeepEqual(t, messages, []string{"helloworld"})
	assert.Assert(t, !tx.Failed())
	messages = nil

	tx.Logf("hello %s!", "world")
	assert.DeepEqual(t, messages, []string{"hello world!"})
	assert.Assert(t, !tx.Failed())
}

func TestErrorErrorf(t *testing.T) {
	messages := []string{}
	logWriter := tester.WithLogWriter(func(name, message string) {
		messages = append(messages, message)
	})
	tx := tester.New(tester.WithName("TestErrorErrorf"), logWriter)

	tester.Run(tx, "In Run", func(tx tester.T) {
		tx.Error("hello", "world")
		tx.Errorf("hello %s!", "world")
	})

	assert.DeepEqual(t, messages, []string{"helloworld", "hello world!"})
	assert.Assert(t, tx.Failed())
}

func TestFatalFatalf(t *testing.T) {
	messages := []string{}
	logWriter := tester.WithLogWriter(func(name, message string) {
		messages = append(messages, message)
	})
	tx := tester.New(tester.WithName("TestFatalFatalff"), logWriter)

	tester.Run(tx, "In Run", func(tx tester.T) {
		tx.Fatal("hello", "world")
		tx.Errorf("hello %s!", "world")
	})

	assert.DeepEqual(t, messages, []string{"helloworld"})
	assert.Assert(t, tx.Failed())
	messages = nil

	tester.Run(tx, "In Run", func(tx tester.T) {
		tx.Fatalf("hello %s!", "world")
		tx.Error("hello", "world")
	})

	assert.DeepEqual(t, messages, []string{"hello world!"})
	assert.Assert(t, tx.Failed())
}

func TestSkip(t *testing.T) {
	messages := []string{}
	logWriter := tester.WithLogWriter(func(name, message string) {
		messages = append(messages, message)
	})
	tx := tester.New(tester.WithName("TestErrorErrorf"), logWriter)

	tester.Run(tx, "In Run", func(tx tester.T) {
		defer func() {
			if tx.Skipped() {
				tx.Log("Skipped!")
			}
		}()
		tx.Skip("hello", "world")
		tx.Errorf("hello %s!", "world")
	})

	assert.DeepEqual(t, messages, []string{"helloworld", "Skipped!"})
	assert.Assert(t, !tx.Failed())
	assert.Assert(t, !tx.Skipped()) // only inner tx is skipped
}

func TestTempDir(t *testing.T) {
	// Note: This has been adapted from
	// https://golang.org/src/testing/testing_test.go

	tx := tester.New(tester.WithName("TestTempDir"), logWriter(t))
	testTempDir(tx)
	tester.Run(tx, "InSubtest", testTempDir)
	tester.Run(tx, "test/subtest", testTempDir)
	tester.Run(tx, "test\\subtest", testTempDir)
	tester.Run(tx, "test:subtest", testTempDir)
	tester.Run(tx, "test/..", testTempDir)
	tester.Run(tx, "../test", testTempDir)
	if r := tester.Results(tx); r.Failed {
		for _, f := range r.Failures {
			t.Error("Failed", f.TestName, f.Failure)
		}
	}
}

func testTempDir(t tester.T) {
	dirCh := make(chan string, 1)
	t.Cleanup(func() {
		// Verify directory has been removed.
		select {
		case dir := <-dirCh:
			fi, err := os.Stat(dir)
			if os.IsNotExist(err) {
				// All good
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Errorf("directory %q stil exists: %v, isDir=%v", dir, fi, fi.IsDir())
		default:
			if !t.Failed() {
				t.Fatal("never received dir channel")
			}
		}
	})

	dir := t.TempDir()
	if dir == "" {
		t.Fatal("expected dir")
	}
	dir2 := t.TempDir()
	if dir == dir2 {
		t.Fatal("subsequent calls to TempDir returned the same directory")
	}
	if filepath.Dir(dir) != filepath.Dir(dir2) {
		t.Fatalf("calls to TempDir do not share a parent; got %q, %q", dir, dir2)
	}
	dirCh <- dir
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.IsDir() {
		t.Errorf("dir %q is not a dir", dir)
	}
	fis, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fis) > 0 {
		t.Errorf("unexpected %d files in TempDir: %v", len(fis), fis)
	}
}

func logWriter(t *testing.T) tester.Option {
	return tester.WithLogWriter(func(name, message string) {
		t.Log(name, message)
	})
}
