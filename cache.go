package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Cache en disco del Markdown ya extraido, por URL. Sirve para abrir
// instantaneamente una pagina ya leida; mientras se lee se refresca en segundo
// plano para la proxima vez. Vive en el directorio de cache del usuario
// (~/.cache/wr en Linux), con permisos 0700/0600.

const (
	cacheVersion = 1 // subir si cambia el formato del Markdown generado
	cacheMaxAge  = 30 * 24 * time.Hour
	cacheTmpAge  = time.Hour
)

func isHTTP(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
}

func cacheDir() (string, error) {
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "wr"), nil
}

// el resultado depende de -L, asi que forma parte de la clave
func cachePath(dir, src string, noLinks bool) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%t|%s", cacheVersion, noLinks, src)))
	return filepath.Join(dir, hex.EncodeToString(sum[:16])+".md")
}

// cacheGet devuelve el Markdown guardado y cuando se guardo.
func cacheGet(src string, noLinks bool) (md string, saved time.Time, ok bool) {
	dir, err := cacheDir()
	if err != nil {
		return "", time.Time{}, false
	}
	data, err := os.ReadFile(cachePath(dir, src, noLinks))
	if err != nil {
		return "", time.Time{}, false
	}
	header, body, found := strings.Cut(string(data), "\n")
	f := strings.SplitN(header, " ", 4)
	if !found || len(f) != 4 || f[0] != "wr-cache" || f[1] != strconv.Itoa(cacheVersion) || f[3] != src {
		return "", time.Time{}, false // formato viejo o colision de clave
	}
	secs, err := strconv.ParseInt(f[2], 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}
	return body, time.Unix(secs, 0), true
}

// cachePut escribe de forma atomica (temporal + rename), asi un refresco
// interrumpido nunca deja una entrada a medias.
func cachePut(src string, noLinks bool, md string) error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "tmp-*")
	if err != nil {
		return err
	}
	_, werr := fmt.Fprintf(tmp, "wr-cache %d %d %s\n%s", cacheVersion, time.Now().Unix(), src, md)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp.Name())
		return werr
	}
	if err := os.Rename(tmp.Name(), cachePath(dir, src, noLinks)); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	pruneCache(dir)
	return nil
}

// pruneCache borra entradas de mas de 30 dias y temporales huerfanos.
func pruneCache(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		age := now.Sub(info.ModTime())
		if age > cacheMaxAge || (strings.HasPrefix(e.Name(), "tmp-") && age > cacheTmpAge) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func cacheClear() error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "hace unos segundos"
	case d < time.Hour:
		return fmt.Sprintf("hace %d min", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("hace %d h", int(d.Hours()))
	}
	return fmt.Sprintf("hace %d dias", int(d.Hours()/24))
}
