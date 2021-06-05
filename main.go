package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
)

type Artist struct {
	Name  string
	Discs []string
}

type ByArtist []Artist

func (a ByArtist) Len() int      { return len(a) }
func (a ByArtist) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByArtist) Less(i, j int) bool {
	return sortName(a[i].Name) < sortName(a[j].Name)
}

type ByName []string

func (a ByName) Len() int      { return len(a) }
func (a ByName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool {
	return sortName(a[i]) < sortName(a[j])
}

type ServeDir struct {
	dir     string
	content string
	sync.Mutex
}

func sortName(name string) string {
	name = strings.ToLower(name)
	for _, prefix := range []string{"the"} {
		prefixSpace := prefix + " "
		if !strings.HasPrefix(name, prefixSpace) {
			continue
		}
		name = strings.TrimPrefix(name, prefixSpace) + ", " + prefix
	}
	return name
}

func sortName(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimPrefix(name, "the ")
	name = strings.TrimSpace(name)
	return name
}

func formatName(name string) string {
	return name
}

func parseId3(dir string) (string, string) {
	dirs, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	for _, fi := range dirs {
		file, err := os.Open(filepath.Join(dir, fi.Name()))
		if err != nil {
			continue
		}
		info, err := tag.ReadFrom(file)
		if err != nil {
			continue
		}
		artist := info.Artist()
		if artist == "" {
			artist = info.AlbumArtist()
		}
		return artist, info.Album()
	}

	return "", ""
}

func loadDiscs(dir string) (string, []string) {
	dirs, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	discs := make([]string, 0, len(dirs))
	var artist string
	for _, fi := range dirs {
		if !fi.IsDir() {
			continue
		}
		var album string
		artist, album = parseId3(filepath.Join(dir, fi.Name()))
		if album == "" {
			album = formatName(fi.Name())
		}
		discs = append(discs, album)
	}
	return artist, discs
}

func loadArtists(dir string) []Artist {
	dirs, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	artists := make([]Artist, 0, len(dirs))
	for _, fi := range dirs {
		if !fi.IsDir() {
			continue
		}
		name, discs := loadDiscs(filepath.Join(dir, fi.Name()))
		sort.Sort(ByName(discs))
		if name == "" {
			name = formatName(fi.Name())
		}
		artists = append(artists, Artist{
			Name:  name,
			Discs: discs,
		})
	}
	sort.Sort(ByArtist(artists))
	return artists
}

func (sd *ServeDir) build() {
	var buf bytes.Buffer

	artists := loadArtists(sd.dir)
	for _, artist := range artists {
		fmt.Fprintln(&buf, artist.Name)
		for _, disc := range artist.Discs {
			fmt.Fprintf(&buf, "    %s\n", disc)
		}
		fmt.Fprintln(&buf, "")
	}
	sd.Lock()
	sd.content = buf.String()
	sd.Unlock()
}

func (sd *ServeDir) builder() {
	for {
		time.Sleep(6 * time.Hour)
		sd.build()
	}
}

func (sd *ServeDir) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sd.Lock()
	content := sd.content
	sd.Unlock()

	w.Write([]byte(content))
}

func main() {
	dir := flag.String("dir", "/home/james/music", "Directory to serve")
	addr := flag.String("addr", ":2002", "Address to serve")
	flag.Parse()

	sd := &ServeDir{
		dir: *dir,
	}
	sd.build()
	go sd.builder()

	log.Fatal(http.ListenAndServe(*addr, sd))
}
