package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// captureStdout 临时把 os.Stdout 换成内部 buffer，执行 fn 后恢复，返回捕获内容。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()

	fn()

	_ = w.Close()
	os.Stdout = orig
	<-done
	return buf.String()
}

// TestOutputJSON 验证 outputJSON 输出缩进 JSON。
func TestOutputJSON(t *testing.T) {
	out := captureStdout(t, func() {
		err := outputJSON(map[string]string{"k": "v"})
		assert.NoError(t, err)
	})
	assert.Contains(t, out, `"k"`)
	assert.Contains(t, out, `"v"`)
	assert.True(t, strings.HasSuffix(out, "\n"), "应以换行结尾")
}

// TestOutputJSON_Error 验证 outputJSON 对不可序列化对象返回错误。
func TestOutputJSON_Error(t *testing.T) {
	err := outputJSON(make(chan int))
	assert.Error(t, err)
}

// TestOutputRaw_NoNewline 验证 outputRaw 在末尾无换行时补换行。
func TestOutputRaw_NoNewline(t *testing.T) {
	out := captureStdout(t, func() { outputRaw("hello") })
	assert.Equal(t, "hello\n", out)
}

// TestOutputRaw_WithNewline 验证 outputRaw 在末尾已有换行时不重复补。
func TestOutputRaw_WithNewline(t *testing.T) {
	out := captureStdout(t, func() { outputRaw("hello\n") })
	assert.Equal(t, "hello\n", out)
}

// TestOutputRaw_Empty 验证空串输出不补换行。
func TestOutputRaw_Empty(t *testing.T) {
	out := captureStdout(t, func() { outputRaw("") })
	assert.Equal(t, "", out)
}

// TestOutputResult_JSON 覆盖 outputResult 的 json（默认）分支。
func TestOutputResult_JSON(t *testing.T) {
	orig := flagOutput
	defer func() { flagOutput = orig }()
	flagOutput = "json"
	out := captureStdout(t, func() {
		err := outputResult(map[string]string{"a": "b"}, "raw ignored", nil)
		assert.NoError(t, err)
	})
	assert.Contains(t, out, `"a"`)
}

// TestOutputResult_Raw 覆盖 outputResult 的 raw 分支。
func TestOutputResult_Raw(t *testing.T) {
	orig := flagOutput
	defer func() { flagOutput = orig }()
	flagOutput = "raw"
	out := captureStdout(t, func() {
		err := outputResult(struct{}{}, "raw payload", nil)
		assert.NoError(t, err)
	})
	assert.Equal(t, "raw payload\n", out)
}

// TestOutputResult_Text 覆盖 outputResult 的 text 分支（含 textFn 非 nil）。
func TestOutputResult_Text(t *testing.T) {
	orig := flagOutput
	defer func() { flagOutput = orig }()
	flagOutput = "text"
	out := captureStdout(t, func() {
		err := outputResult(struct{}{}, "", func() string { return "text summary" })
		assert.NoError(t, err)
	})
	assert.Equal(t, "text summary\n", out)
}

// TestOutputResult_TextNilFn 覆盖 outputResult 的 text 分支且 textFn 为 nil。
func TestOutputResult_TextNilFn(t *testing.T) {
	orig := flagOutput
	defer func() { flagOutput = orig }()
	flagOutput = "text"
	out := captureStdout(t, func() {
		err := outputResult(struct{}{}, "", nil)
		assert.NoError(t, err)
	})
	assert.Equal(t, "", out)
}

// TestErrExit_Text 子进程隔离测试 errExit 文本格式分支。
// 子进程 errExit 走 logrus.Errorf（输出到 stderr）后 os.Exit(1)。
func TestErrExit_Text(t *testing.T) {
	if os.Getenv("BE_TEST_ERREXIT_TEXT") == "1" {
		flagLogFormat = "text"
		errExit(&cobra.Command{}, "boom text")
		return
	}
	cmd, _, stderr := execTestSelf(t, "BE_TEST_ERREXIT_TEXT=1")
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, 1, ee.ExitCode())
	} else {
		t.Fatalf("期望 *exec.ExitError exit 1, got %v", err)
	}
	assert.Contains(t, stderr.String(), "boom text")
}

// TestErrExit_JSON 子进程隔离测试 errExit JSON 格式分支。
// 子进程 errExit 走 outputJSON（输出到 stdout）后 os.Exit(1)。
func TestErrExit_JSON(t *testing.T) {
	if os.Getenv("BE_TEST_ERREXIT_JSON") == "1" {
		flagLogFormat = "json"
		errExit(&cobra.Command{}, "boom json")
		return
	}
	cmd, stdout, _ := execTestSelf(t, "BE_TEST_ERREXIT_JSON=1")
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, 1, ee.ExitCode())
	} else {
		t.Fatalf("期望 *exec.ExitError exit 1, got %v", err)
	}
	assert.Contains(t, stdout.String(), "boom json")
	assert.Contains(t, stdout.String(), "error")
}
