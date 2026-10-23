package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseShow(t *testing.T) {
	got := parseShow([]byte(`Package: vlc
Version: 3.0.23-1
Installed-Size: 195
Size: 36130
Homepage: https://www.videolan.org/vlc/
Description-en: multimedia player and streamer
 VLC is the VideoLAN project's media player.
 .
 It plays almost anything.

Package: nothing-else
Version: 1
Description: another
`))
	vlc := got["vlc"]
	if vlc.version != "3.0.23-1" || vlc.size != 36130 || vlc.installedSize != 195*1024 || vlc.homepage == "" {
		t.Fatalf("vlc = %+v", vlc)
	}
	if vlc.summary != "multimedia player and streamer" || !strings.Contains(vlc.description, "media player.\n\nIt plays") {
		t.Fatalf("description = %q / %q", vlc.summary, vlc.description)
	}
	if got["nothing-else"].summary != "another" {
		t.Fatalf("second stanza = %+v", got["nothing-else"])
	}
}

func TestInstalledStateAndSearchRanking(t *testing.T) {
	installed := parseInstalled([]byte("vlc\tii \t3.0.23-1\nold\trc \t1\nheld\thi \t2\n"))
	if installed["vlc"] != "3.0.23-1" || installed["held"] != "2" || installed["old"] != "" {
		t.Fatalf("installed = %v", installed)
	}

	terms := []string{"vlc"}
	hits := parseSearch([]byte(`libvlc-dev - development files
vlc-plugin-qt - plugin for vlc
vlc - multimedia player
unrelated - a tool that mentions nothing
`), terms)
	items := rankSearch(hits, terms, installed)
	if len(items) != 2 || items[0].ID != "vlc" || !items[0].Installed || items[1].ID != "vlc-plugin-qt" {
		t.Fatalf("ranked = %+v", items)
	}
}

// A stand-in for zenith-pkg-add: the three outcomes the store has to tell apart.
func TestPackageHelperOutcomes(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case $2 in
ok-pkg) echo 'dlstatus:0:50.0000:Retrieving'; echo 'pmstatus:ok-pkg:100.0000:Setting up'; echo 'Reading package lists' ;;
denied-pkg) exit 127 ;;
refused-pkg) echo 'zenith-pkg-add: this would also remove: ghostty' >&2; exit 13 ;;
slow-pkg) echo 'dlstatus:0:5.0000:Retrieving'; read -r line; [ "$line" = cancel ] && exit 143 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "zenith-pkg-add"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out := func(f func()) string {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		f()
		w.Close()
		os.Stdout = old
		b, _ := io.ReadAll(r)
		return string(b)
	}

	var err error
	lines := out(func() { err = runPackageHelper(context.Background(), "zenith-pkg-add", "INSTALLING", "ok-pkg") })
	if err != nil {
		t.Fatalf("ok-pkg: %v", err)
	}
	want := `{"stage":"AUTHORIZING","percent":-1}` + "\n" + `{"stage":"DOWNLOADING","percent":50}` + "\n" + `{"stage":"INSTALLING","percent":100}` + "\n"
	if lines != want {
		t.Fatalf("stages = %q, want %q", lines, want)
	}

	out(func() { err = runPackageHelper(context.Background(), "zenith-pkg-add", "INSTALLING", "denied-pkg") })
	if err == nil || !strings.Contains(err.Error(), "authorization was declined") {
		t.Fatalf("denied-pkg error = %v", err)
	}

	out(func() { err = runPackageHelper(context.Background(), "zenith-pkg-add", "INSTALLING", "refused-pkg") })
	if err == nil || err.Error() != "this would also remove: ghostty" {
		t.Fatalf("refused-pkg error = %v, want the helper's sentence without its name", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	out(func() { err = runPackageHelper(ctx, "zenith-pkg-add", "INSTALLING", "slow-pkg") })
	if err != errCancelled {
		t.Fatalf("slow-pkg error = %v, want cancelled", err)
	}

	if err := runPackageHelper(context.Background(), "zenith-pkg-add", "INSTALLING", "-oops"); err == nil {
		t.Fatal("a name that looks like an option reached the helper")
	}
}
