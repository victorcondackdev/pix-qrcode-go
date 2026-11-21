package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const payload = "00020126330014br.gov.bcb.pix0111123456789095204000053039865406150.005802BR" +
	"5912LOJA EXEMPLO6009SAO PAULO62140510PEDIDO123463047DEC"

func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestTerminalDefault(t *testing.T) {
	code, out, _ := runCLI(t, "", payload)
	if code != exitOK || strings.Count(out, "\n") != (37+8+1)/2 || !strings.Contains(out, "█") {
		t.Fatalf("exit %d, output:\n%s", code, out)
	}
	_, inv, _ := runCLI(t, "  "+payload+"\n", "--invert", "--quiet", "1", "-")
	if !strings.HasPrefix(inv, "█▀▀▀▀▀▀▀█") {
		t.Fatalf("stdin + invert: %q", strings.SplitN(inv, "\n", 2)[0])
	}
}

func TestFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.png")
	if code, out, errs := runCLI(t, "", "-o", p, "--size", "300", "--level", "h", payload); code != exitOK || out != "" {
		t.Fatalf("exit %d %q %q", code, out, errs)
	}
	f, _ := os.Open(p)
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil || img.Bounds().Dx() != 300 {
		t.Fatalf("png: %v %v", err, img.Bounds())
	}
	s := filepath.Join(dir, "out.svg")
	if code, _, _ := runCLI(t, "", "-o", s, "--term", payload); code != exitOK {
		t.Fatal(code)
	}
	b, _ := os.ReadFile(s)
	if !bytes.HasPrefix(b, []byte("<svg")) {
		t.Fatalf("%.40s", b)
	}
	s2 := filepath.Join(dir, "x.img")
	runCLI(t, "", "-o", s2, "--svg", payload)
	if b, _ := os.ReadFile(s2); !bytes.HasPrefix(b, []byte("<svg")) {
		t.Fatal("--svg with -o")
	}
	if code, out, _ := runCLI(t, "", "--svg", payload); code != exitOK || !strings.HasPrefix(out, "<svg") || !strings.HasSuffix(out, "</svg>\n") {
		t.Fatalf("svg to stdout: %d %.40s", code, out)
	}
	if code, _, _ := runCLI(t, "", "-o", filepath.Join(dir, "missing", "x.png"), payload); code != exitInvalid {
		t.Fatal("unwritable path")
	}
}

func TestInfo(t *testing.T) {
	code, out, _ := runCLI(t, "", "--info", payload)
	for _, want := range []string{"version   5 (37x37 modules)", "level     M", "data      688 bits (byte mode: 1044 bits, version 8)", "segment   alphanumeric  64"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if code != exitOK || strings.Contains(out, "█") {
		t.Fatal("--info alone must not print the code")
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		args  []string
		stdin string
		code  int
		msg   string
	}{
		{[]string{"--level", "X", payload}, "", exitUsage, "--level"},
		{[]string{"--bogus"}, "", exitUsage, "flag provided but not defined"},
		{[]string{"a", "b"}, "", exitUsage, "expected one payload"},
		{nil, "   ", exitUsage, "no payload"},
		{[]string{"hello"}, "", exitInvalid, "not a BR Code"},
		{[]string{payload[:len(payload)-1] + "0"}, "", exitInvalid, "CRC mismatch"},
		{[]string{"--size", "10", "-o", filepath.Join(t.TempDir(), "a.png"), payload}, "", exitUsage, "cannot hold"},
		{[]string{"--quiet", "-1", payload}, "", exitUsage, "quiet zone"},
		{[]string{"--quiet", "-1", "--svg", payload}, "", exitUsage, "quiet zone"},
		{[]string{"--no-validate", "--level", "H", strings.Repeat("a", 1300)}, "", exitInvalid, "too long"},
		{[]string{"--no-validate", "--info", "--level", "H", strings.Repeat("1", 3000)}, "", exitInvalid, "too long"},
	}
	for _, tt := range tests {
		code, _, errs := runCLI(t, tt.stdin, tt.args...)
		if code != tt.code || !strings.Contains(errs, tt.msg) {
			t.Errorf("%v: exit %d, stderr %q", tt.args, code, errs)
		}
	}
	if code, out, _ := runCLI(t, "", "-h"); code != exitOK || !strings.HasPrefix(out, "pixqr renders") {
		t.Fatal("-h")
	}
	if code, out, _ := runCLI(t, "", "--no-validate", "hello"); code != exitOK || out == "" {
		t.Fatal("--no-validate")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, os.ErrClosed }

func TestStdinError(t *testing.T) {
	var out, errb bytes.Buffer
	if run(nil, errReader{}, &out, &errb) != exitInvalid {
		t.Fatal(errb.String())
	}
}
