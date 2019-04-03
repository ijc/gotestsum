package junitxml

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/assert"
	"gotest.tools/env"
	"gotest.tools/golden"
	"gotest.tools/gotestsum/testjson"
)

func TestMungePackageName(t *testing.T) {
	in := "a/b/c/d/e/f"
	for _, tc := range []struct {
		name   string
		strip  int
		prefix string
		exp    string
	}{
		{name: "identity", exp: "a/b/c/d/e/f"},
		{name: "strip1", strip: 1, exp: "b/c/d/e/f"},
		{name: "strip3", strip: 3, exp: "d/e/f"},
		{name: "strip_most", strip: 5, exp: "f"},
		{name: "strip_all", strip: 6, exp: ""},
		{name: "strip_too_many", strip: 7, exp: ""},
		{name: "prefix", prefix: "1/2/3", exp: "1/2/3/a/b/c/d/e/f"},
		{name: "prefix_trailing", prefix: "1/2/3/", exp: "1/2/3/a/b/c/d/e/f"},
		{name: "strip_and_prefix", strip: 3, prefix: "1/2/3", exp: "1/2/3/d/e/f"},
		{name: "strip_all_prefix", strip: 6, prefix: "1/2/3", exp: "1/2/3"},
		{name: "strip_too_many_prefix", strip: 7, prefix: "1/2/3", exp: "1/2/3"},
	} {

		t.Run(tc.name, func(t *testing.T) {
			act := mungePackageName(in, tc.strip, tc.prefix)
			assert.Equal(t, tc.exp, act)
		})
	}
}

func TestWrite(t *testing.T) {
	exec := createExecution(t)

	expected := string(golden.Get(t, "junitxml-report.golden"))
	defer env.Patch(t, "GOVERSION", "go7.7.7")()

	t.Run("base", func(t *testing.T) {
		out := new(bytes.Buffer)
		err := Write(out, exec, 0, "")
		assert.NilError(t, err)
		assert.Equal(t, out.String(), expected)
	})

	t.Run("strip", func(t *testing.T) {
		out := new(bytes.Buffer)
		err := Write(out, exec, 2, "")
		assert.NilError(t, err)
		// Replacement is anchored with " to avoid substitution in error messages.
		expected := strings.Replace(expected, `"github.com/gotestyourself/`, `"`, -1)
		assert.Equal(t, out.String(), expected)
	})

	t.Run("prefix", func(t *testing.T) {
		out := new(bytes.Buffer)
		err := Write(out, exec, 0, "a/b/c")
		assert.NilError(t, err)
		// Replacement is anchored with " to avoid substitution in error messages.
		expected := strings.Replace(expected, `"github.com/gotestyourself/`, `"a/b/c/github.com/gotestyourself/`, -1)
		// Empty classnames also get prefixed.
		expected = strings.Replace(expected, `classname=""`, `classname="a/b/c"`, -1)
		assert.Equal(t, out.String(), expected)
	})
}

func createExecution(t *testing.T) *testjson.Execution {
	exec, err := testjson.ScanTestOutput(testjson.ScanConfig{
		Stdout:  readTestData(t, "out"),
		Stderr:  readTestData(t, "err"),
		Handler: &noopHandler{},
	})
	assert.NilError(t, err)
	return exec
}

func readTestData(t *testing.T, stream string) io.Reader {
	raw, err := ioutil.ReadFile("../../testjson/testdata/go-test-json." + stream)
	assert.NilError(t, err)
	return bytes.NewReader(raw)
}

type noopHandler struct{}

func (s *noopHandler) Event(testjson.TestEvent, *testjson.Execution) error {
	return nil
}

func (s *noopHandler) Err(string) error {
	return nil
}

func TestGoVersion(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		defer env.Patch(t, "PATH", "/bogus")()
		assert.Equal(t, goVersion(), "unknown")
	})

	t.Run("current version", func(t *testing.T) {
		expected := fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		assert.Equal(t, goVersion(), expected)
	})
}
